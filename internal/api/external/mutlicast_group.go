package external

import (
	"crypto/aes"
	"crypto/rand"
	"github.com/brocaar/lorawan/applayer/multicastsetup"
	"github.com/gofrs/uuid"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"strconv"
	"strings"
	"time"

	"github.com/brocaar/lorawan"
	pb "github.com/gyh1621/chirpstack-api/go/v3/as/external/api"
	"github.com/gyh1621/chirpstack-api/go/v3/ns"
	"github.com/gyh1621/chirpstack-application-server/internal/api/external/auth"
	"github.com/gyh1621/chirpstack-application-server/internal/api/helpers"
	"github.com/gyh1621/chirpstack-application-server/internal/backend/networkserver"
	"github.com/gyh1621/chirpstack-application-server/internal/multicast"
	"github.com/gyh1621/chirpstack-application-server/internal/storage"
)

// MulticastGroupAPI implements the multicast-group api.
type MulticastGroupAPI struct {
	validator        auth.Validator
	routingProfileID uuid.UUID
}

// NewMulticastGroupAPI creates a new multicast-group API.
func NewMulticastGroupAPI(validator auth.Validator, routingProfileID uuid.UUID) *MulticastGroupAPI {
	return &MulticastGroupAPI{
		validator:        validator,
		routingProfileID: routingProfileID,
	}
}

// Create creates the given multicast-group.
func (a *MulticastGroupAPI) Create(ctx context.Context, req *pb.CreateMulticastGroupRequest) (*pb.CreateMulticastGroupResponse, error) {
	if req.MulticastGroup == nil {
		return nil, grpc.Errorf(codes.InvalidArgument, "multicast_group must not be nil")
	}

	spID, err := uuid.FromString(req.MulticastGroup.ServiceProfileId)
	if err != nil {
		return nil, grpc.Errorf(codes.InvalidArgument, err.Error())
	}

	sp, err := storage.GetServiceProfile(ctx, storage.DB(), spID, true) // local-only, as we only want to fetch the org. id
	if err != nil {
		return nil, helpers.ErrToRPCError(err)
	}

	if err = a.validator.Validate(ctx,
		auth.ValidateMulticastGroupsAccess(auth.Create, sp.OrganizationID)); err != nil {
		return nil, grpc.Errorf(codes.Unauthenticated, "authentication failed: %s", err)
	}

	var mcAddr lorawan.DevAddr
	err = mcAddr.UnmarshalText([]byte(req.MulticastGroup.McAddr))
	if err != nil || len(req.MulticastGroup.McAddr) == 0 {
		if _, err1 := rand.Read(mcAddr[:]); err1 != nil {
			return nil, grpc.Errorf(codes.Unknown, "read random bytes error")
		}
	}

	var mcKey lorawan.AES128Key
	if _, err := rand.Read(mcKey[:]); err != nil {
		return nil, grpc.Errorf(codes.InvalidArgument, "get mcKey error")
	}

	mcNetSKey, err := multicastsetup.GetMcNetSKey(mcKey, mcAddr)
	if err != nil {
		return nil, grpc.Errorf(codes.Unknown, "get McNetSKey error")
	}

	mg := storage.MulticastGroup{
		Name:             req.MulticastGroup.Name,
		ServiceProfileID: spID,
		MCKey:            mcKey,
		MulticastGroup: ns.MulticastGroup{
			McAddr:           mcAddr[:],
			McNwkSKey:        mcNetSKey[:],
			GroupType:        ns.MulticastGroupType(req.MulticastGroup.GroupType),
			Dr:               req.MulticastGroup.Dr,
			Frequency:        req.MulticastGroup.Frequency,
			PingSlotPeriod:   req.MulticastGroup.PingSlotPeriod,
			ServiceProfileId: spID.Bytes(),
			RoutingProfileId: a.routingProfileID.Bytes(),
			FCnt:             req.MulticastGroup.FCnt,
		},
	}

	mcAppSKey, err := multicastsetup.GetMcAppSKey(mcKey, mcAddr)
	if err != nil {
		return nil, grpc.Errorf(codes.InvalidArgument, "get McAppSKey error: %s", err)
	}
	mg.MCAppSKey = mcAppSKey

	if err = storage.Transaction(func(tx sqlx.Ext) error {
		if err := storage.CreateMulticastGroup(ctx, tx, &mg); err != nil {
			return helpers.ErrToRPCError(err)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	var mgID uuid.UUID
	copy(mgID[:], mg.MulticastGroup.Id)

	log.Infof("Created a multicast group, McKey: %s, McNwkSKey: %s, McAppSKey: %s, McAddr: %s",
		mg.MCKey, mcNetSKey, mg.MCAppSKey, mcAddr,
	)

	return &pb.CreateMulticastGroupResponse{
		Id: mgID.String(),
	}, nil
}

// Get returns a multicast-group given an ID.
func (a *MulticastGroupAPI) Get(ctx context.Context, req *pb.GetMulticastGroupRequest) (*pb.GetMulticastGroupResponse, error) {
	mgID, err := uuid.FromString(req.Id)
	if err != nil {
		return nil, grpc.Errorf(codes.InvalidArgument, "id: %s", err)
	}

	if err = a.validator.Validate(ctx,
		auth.ValidateMulticastGroupAccess(auth.Read, mgID)); err != nil {
		return nil, grpc.Errorf(codes.Unauthenticated, "authentication failed: %s", err)
	}

	mg, err := storage.GetMulticastGroup(ctx, storage.DB(), mgID, false, false)
	if err != nil {
		return nil, helpers.ErrToRPCError(err)
	}

	var mcAddr lorawan.DevAddr
	copy(mcAddr[:], mg.MulticastGroup.McAddr)

	out := pb.GetMulticastGroupResponse{
		MulticastGroup: &pb.MulticastGroup{
			Id:               mgID.String(),
			Name:             mg.Name,
			McAddr:           mcAddr.String(),
			FCnt:             mg.MulticastGroup.FCnt,
			GroupType:        pb.MulticastGroupType(mg.MulticastGroup.GroupType),
			Dr:               mg.MulticastGroup.Dr,
			Frequency:        mg.MulticastGroup.Frequency,
			PingSlotPeriod:   mg.MulticastGroup.PingSlotPeriod,
			ServiceProfileId: mg.ServiceProfileID.String(),
		},
	}

	out.CreatedAt, err = ptypes.TimestampProto(mg.CreatedAt)
	if err != nil {
		return nil, helpers.ErrToRPCError(err)
	}

	out.UpdatedAt, err = ptypes.TimestampProto(mg.UpdatedAt)
	if err != nil {
		return nil, helpers.ErrToRPCError(err)
	}

	return &out, nil
}

// Update updates the given multicast-group.
func (a *MulticastGroupAPI) Update(ctx context.Context, req *pb.UpdateMulticastGroupRequest) (*empty.Empty, error) {
	if req.MulticastGroup == nil {
		return nil, grpc.Errorf(codes.InvalidArgument, "multicast_group must not be nil")
	}

	mgID, err := uuid.FromString(req.MulticastGroup.Id)
	if err != nil {
		return nil, grpc.Errorf(codes.InvalidArgument, "id: %s", err)
	}

	if err = a.validator.Validate(ctx,
		auth.ValidateMulticastGroupAccess(auth.Update, mgID)); err != nil {
		return nil, grpc.Errorf(codes.Unauthenticated, "authentication failed: %s", err)
	}

	mg, err := storage.GetMulticastGroup(ctx, storage.DB(), mgID, false, false)
	if err != nil {
		return nil, helpers.ErrToRPCError(err)
	}

	var mcAddr lorawan.DevAddr
	if err = mcAddr.UnmarshalText([]byte(req.MulticastGroup.McAddr)); err != nil {
		return nil, grpc.Errorf(codes.InvalidArgument, "mc_app_s_key: %s", err)
	}

	mg.Name = req.MulticastGroup.Name
	mg.MulticastGroup = ns.MulticastGroup{
		Id:               mg.MulticastGroup.Id,
		McAddr:           mg.MulticastGroup.McAddr[:],
		McNwkSKey:        mg.MulticastGroup.McNwkSKey[:],
		GroupType:        ns.MulticastGroupType(req.MulticastGroup.GroupType),
		Dr:               req.MulticastGroup.Dr,
		Frequency:        req.MulticastGroup.Frequency,
		PingSlotPeriod:   req.MulticastGroup.PingSlotPeriod,
		ServiceProfileId: mg.MulticastGroup.ServiceProfileId,
		RoutingProfileId: mg.MulticastGroup.RoutingProfileId,
		FCnt:             req.MulticastGroup.FCnt,
	}

	if err = storage.Transaction(func(tx sqlx.Ext) error {
		if err := storage.UpdateMulticastGroup(ctx, tx, &mg); err != nil {
			return helpers.ErrToRPCError(err)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return &empty.Empty{}, nil
}

// Delete deletes a multicast-group given an ID.
func (a *MulticastGroupAPI) Delete(ctx context.Context, req *pb.DeleteMulticastGroupRequest) (*empty.Empty, error) {
	mgID, err := uuid.FromString(req.Id)
	if err != nil {
		return nil, grpc.Errorf(codes.InvalidArgument, "id: %s", err)
	}

	if err = storage.Transaction(func(tx sqlx.Ext) error {
		if err := storage.DeleteMulticastGroup(ctx, tx, mgID); err != nil {
			return helpers.ErrToRPCError(err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return &empty.Empty{}, nil
}

// List lists the available multicast-groups.
func (a *MulticastGroupAPI) List(ctx context.Context, req *pb.ListMulticastGroupRequest) (*pb.ListMulticastGroupResponse, error) {
	var err error
	var idFilter bool

	filters := storage.MulticastGroupFilters{
		OrganizationID: req.OrganizationId,
		Search:         req.Search,
		Limit:          int(req.Limit),
		Offset:         int(req.Offset),
	}

	// if org. filter has been set, validate the client has access to the given org
	if filters.OrganizationID != 0 {
		idFilter = true

		if err = a.validator.Validate(ctx,
			auth.ValidateOrganizationAccess(auth.Read, req.OrganizationId)); err != nil {
			return nil, grpc.Errorf(codes.Unauthenticated, "authentication failed: %s", err)
		}
	}

	// if sp filter has been set, validate the client has access to the given sp
	if req.ServiceProfileId != "" {
		idFilter = true

		filters.ServiceProfileID, err = uuid.FromString(req.ServiceProfileId)
		if err != nil {
			return nil, grpc.Errorf(codes.InvalidArgument, "service_profile_id: %s", err)
		}

		if err = a.validator.Validate(ctx,
			auth.ValidateServiceProfileAccess(auth.Read, filters.ServiceProfileID)); err != nil {
			return nil, grpc.Errorf(codes.Unauthenticated, "authentication error: %s", err)
		}
	}

	// if devEUI has been set, validate the client has access to the given device
	if req.DevEui != "" {
		idFilter = true

		if err = filters.DevEUI.UnmarshalText([]byte(req.DevEui)); err != nil {
			return nil, grpc.Errorf(codes.InvalidArgument, "dev_eui: %s", err)
		}

		if err = a.validator.Validate(ctx,
			auth.ValidateNodeAccess(filters.DevEUI, auth.Read)); err != nil {
			return nil, grpc.Errorf(codes.Unauthenticated, "authentication error: %s", err)
		}
	}

	// listing all stored objects is for global admin only
	if !idFilter {
		user, err := a.validator.GetUser(ctx)
		if err != nil {
			return nil, helpers.ErrToRPCError(err)
		}

		if !user.IsAdmin {
			return nil, grpc.Errorf(codes.Unauthenticated, "client must be global admin for unfiltered request")
		}
	}

	count, err := storage.GetMulticastGroupCount(ctx, storage.DB(), filters)
	if err != nil {
		return nil, helpers.ErrToRPCError(err)
	}

	items, err := storage.GetMulticastGroups(ctx, storage.DB(), filters)
	if err != nil {
		return nil, helpers.ErrToRPCError(err)
	}

	out := pb.ListMulticastGroupResponse{
		TotalCount: int64(count),
	}

	for _, item := range items {
		out.Result = append(out.Result, &pb.MulticastGroupListItem{
			Id:                 item.ID.String(),
			Name:               item.Name,
			ServiceProfileId:   item.ServiceProfileID.String(),
			ServiceProfileName: item.ServiceProfileName,
		})
	}

	return &out, nil
}

// AddDevice adds the given device to the multicast-group.
func (a *MulticastGroupAPI) AddDevice(ctx context.Context, req *pb.AddDeviceToMulticastGroupRequest) (*empty.Empty, error) {
	mgID, err := uuid.FromString(req.MulticastGroupId)
	if err != nil {
		return nil, grpc.Errorf(codes.InvalidArgument, "multicast_group_id: %s", err)
	}

	var devEUI lorawan.EUI64
	if err = devEUI.UnmarshalText([]byte(req.DevEui)); err != nil {
		return nil, grpc.Errorf(codes.InvalidArgument, "dev_eui: %s", err)
	}

	if err = a.validator.Validate(ctx,
		auth.ValidateMulticastGroupAccess(auth.Update, mgID)); err != nil {
		return nil, grpc.Errorf(codes.Unauthenticated, "authentication failed: %s", err)
	}

	// get next available McGroupID for this device
	mcGroupID := -1
	existedRMSs, err := storage.GetRemoteMulticastSetupItemsByDevice(ctx, storage.DB(), devEUI)
	if len(existedRMSs) != 0 {
		existedIDs := make([]bool, 4)
		for _, rms := range existedRMSs {
			existedIDs[rms.McGroupID] = true
		}
		for i := 0; i < 4; i++ {
			if !existedIDs[i] {
				mcGroupID = i
				break
			}
		}
		if mcGroupID == -1 {
			// no available id left
			return nil, grpc.Errorf(codes.Unavailable, "Number of groups the device can join reaches maximum")
		}
	} else {
		mcGroupID = 0
	}

	// validate that the device is under the same service-profile as the multicast-group
	dev, err := storage.GetDevice(ctx, storage.DB(), devEUI, false, true)
	if err != nil {
		return nil, helpers.ErrToRPCError(err)
	}

	dp, err := storage.GetDeviceProfile(ctx, storage.DB(), dev.DeviceProfileID, false, false)
	if err != nil {
		return nil, helpers.ErrToRPCError(err)
	}

	app, err := storage.GetApplication(ctx, storage.DB(), dev.ApplicationID)
	if err != nil {
		return nil, helpers.ErrToRPCError(err)
	}

	mg, err := storage.GetMulticastGroup(ctx, storage.DB(), mgID, false, false)
	if err != nil {
		return nil, helpers.ErrToRPCError(err)
	}

	if app.ServiceProfileID != mg.ServiceProfileID {
		return nil, grpc.Errorf(codes.FailedPrecondition, "service-profile of device != service-profile of multicast-group")
	}

	dk, err := storage.GetDeviceKeys(ctx, storage.DB(), devEUI)
	if err != nil {
		return nil, helpers.ErrToRPCError(err)
	}

	var nullKey lorawan.AES128Key

	// get the encrypted McKey.
	log.Info("Muticast Ket Generating Start")
	var mcKeyEncrypted, mcRootKey lorawan.AES128Key
	devMacVersion, err := strconv.Atoi(strings.ReplaceAll(dp.DeviceProfile.MacVersion, ".", ""))
	if err != nil {
		return nil, helpers.ErrToRPCError(err)
	}
	if dk.AppKey != nullKey && devMacVersion >= 110 {
		mcRootKey, err = multicastsetup.GetMcRootKeyForAppKey(dk.AppKey)
		if err != nil {
			return nil, grpc.Errorf(codes.Unknown, "get McRootKey for AppKey error", err)
		}
		log.Infof("Use AppKey to generate, appkey: %s, mcRootKey: %s", dk.AppKey, mcRootKey)
	} else {
		mcRootKey, err = multicastsetup.GetMcRootKeyForGenAppKey(dk.GenAppKey)
		if err != nil {
			return nil, grpc.Errorf(codes.Unknown, "get McRootKey for GenAppKey error", err)
		}
		log.Infof("Use GenAppKey to generate, genappkey: %s, mcRootKey: %s", dk.GenAppKey, mcRootKey)
	}

	mcKEKey, err := multicastsetup.GetMcKEKey(mcRootKey)
	log.Infof("Get McKEKey: %s", mcKEKey)
	if err != nil {
		return nil, grpc.Errorf(codes.Unknown, "get McKEKey error", err)
	}

	block, err := aes.NewCipher(mcKEKey[:])
	if err != nil {
		return nil, grpc.Errorf(codes.Unknown, "new cipher error", err)
	}
	block.Encrypt(mcKeyEncrypted[:], mg.MCKey[:])
	log.Infof("Get McKey_Encrypted: %s", mcKeyEncrypted)
	log.Infof("McKey is: %s", mg.MCKey)

	var mcAddr lorawan.DevAddr
	copy(mcAddr[:], mg.MulticastGroup.McAddr)
	mcNetSKey, err := multicastsetup.GetMcNetSKey(mg.MCKey, mcAddr)
	mcAppSKey, err := multicastsetup.GetMcAppSKey(mg.MCKey, mcAddr)
	log.Infof("McAddr: %s, mg.mcaddr: %s", mcAddr, string(mg.MulticastGroup.McAddr))
	log.Infof("Generated NetKey: %s", mcNetSKey)
	log.Infof("Generated AppKey: %s", mcAppSKey)

	// create remote multicast setup record for device
	rms := storage.RemoteMulticastSetup{
		DevEUI:           dk.DevEUI,
		MulticastGroupID: mgID,
		McGroupID:        mcGroupID,
		McKeyEncrypted:   mcKeyEncrypted,
		MinMcFCnt:        0,
		MaxMcFCnt:        (1 << 32) - 1,
		State:            storage.RemoteMulticastSetupSetup,
		RetryInterval:    time.Second * 30,
	}
	copy(rms.McAddr[:], mg.MulticastGroup.McAddr)
	log.Infof("remote multicast logs, before create, mcAddr: %s, %s",
		string(rms.McAddr[:]), mg.MulticastGroup.McAddr)

	err = storage.CreateRemoteMulticastSetup(ctx, storage.DB(), &rms)
	if err != nil {
		return nil, grpc.Errorf(codes.Unknown, "create remote multicast setup error: %s", err)
	}

	return &empty.Empty{}, nil
}

// RemoveDevice removes the given device from the multicast-group.
func (a *MulticastGroupAPI) RemoveDevice(ctx context.Context, req *pb.RemoveDeviceFromMulticastGroupRequest) (*empty.Empty, error) {
	mgID, err := uuid.FromString(req.MulticastGroupId)
	if err != nil {
		return nil, grpc.Errorf(codes.InvalidArgument, "multicast_group_id: %s", err)
	}

	var devEUI lorawan.EUI64
	if err = devEUI.UnmarshalText([]byte(req.DevEui)); err != nil {
		return nil, grpc.Errorf(codes.InvalidArgument, "dev_eui: %s", err)
	}

	if err = a.validator.Validate(ctx,
		auth.ValidateMulticastGroupAccess(auth.Update, mgID)); err != nil {
		return nil, grpc.Errorf(codes.Unauthenticated, "authentication failed: %s", err)
	}

	rms, err := storage.GetRemoteMulticastSetup(ctx, storage.DB(), devEUI, mgID, true)
	if err != nil {
		return nil, grpc.Errorf(codes.Unknown, "get remote multiast-setup error: %s", err)
	}
	if rms.State == storage.RemoteMulticastSetupDelete {
		return nil, grpc.Errorf(codes.AlreadyExists, "delete request is already enqueued")
	}
	rms.RetryCount = 0
	rms.RetryAfter = time.Now()
	rms.State = storage.RemoteMulticastSetupDelete
	rms.StateProvisioned = false
	if err = storage.UpdateRemoteMulticastSetup(ctx, storage.DB(), &rms); err != nil {
		return nil, grpc.Errorf(codes.Unknown, "update remote multiast-setup error: %s", err)
	}

	return &empty.Empty{}, nil
}

// Enqueue adds the given item to the multicast-queue.
func (a *MulticastGroupAPI) Enqueue(ctx context.Context, req *pb.EnqueueMulticastQueueItemRequest) (*pb.EnqueueMulticastQueueItemResponse, error) {
	var fCnt uint32

	if req.MulticastQueueItem == nil {
		return nil, grpc.Errorf(codes.InvalidArgument, "multicast_queue_item must not be nil")
	}

	if req.MulticastQueueItem.FPort == 0 {
		return nil, grpc.Errorf(codes.InvalidArgument, "f_port must be > 0")
	}

	mgID, err := uuid.FromString(req.MulticastQueueItem.MulticastGroupId)
	if err != nil {
		return nil, grpc.Errorf(codes.InvalidArgument, "multicast_group_id: %s", err)
	}

	if err = a.validator.Validate(ctx,
		auth.ValidateMulticastGroupQueueAccess(auth.Create, mgID)); err != nil {
		return nil, grpc.Errorf(codes.Unauthenticated, "authentication failed: %s", err)
	}

	if err = storage.Transaction(func(tx sqlx.Ext) error {
		var err error
		fCnt, err = multicast.Enqueue(ctx, tx, mgID, uint8(req.MulticastQueueItem.FPort), req.MulticastQueueItem.Data)
		if err != nil {
			return grpc.Errorf(codes.Internal, "enqueue multicast-group queue-item error: %s", err)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return &pb.EnqueueMulticastQueueItemResponse{
		FCnt: fCnt,
	}, nil
}

// FlushQueue flushes the multicast-group queue.
func (a *MulticastGroupAPI) FlushQueue(ctx context.Context, req *pb.FlushMulticastGroupQueueItemsRequest) (*empty.Empty, error) {
	mgID, err := uuid.FromString(req.MulticastGroupId)
	if err != nil {
		return nil, grpc.Errorf(codes.InvalidArgument, "multicast_group_id: %s", err)
	}

	if err = a.validator.Validate(ctx,
		auth.ValidateMulticastGroupQueueAccess(auth.Delete, mgID)); err != nil {
		return nil, grpc.Errorf(codes.Unauthenticated, "authentication failed: %s", err)
	}

	n, err := storage.GetNetworkServerForMulticastGroupID(ctx, storage.DB(), mgID)
	if err != nil {
		return nil, helpers.ErrToRPCError(err)
	}

	nsClient, err := networkserver.GetPool().Get(n.Server, []byte(n.CACert), []byte(n.TLSCert), []byte(n.TLSKey))
	if err != nil {
		return nil, helpers.ErrToRPCError(err)
	}

	_, err = nsClient.FlushMulticastQueueForMulticastGroup(ctx, &ns.FlushMulticastQueueForMulticastGroupRequest{
		MulticastGroupId: mgID.Bytes(),
	})
	if err != nil {
		return nil, err
	}

	return &empty.Empty{}, nil
}

// ListQueue lists the items in the multicast-group queue.
func (a *MulticastGroupAPI) ListQueue(ctx context.Context, req *pb.ListMulticastGroupQueueItemsRequest) (*pb.ListMulticastGroupQueueItemsResponse, error) {
	mgID, err := uuid.FromString(req.MulticastGroupId)
	if err != nil {
		return nil, grpc.Errorf(codes.InvalidArgument, "multicast_group_id: %s", err)
	}

	if err = a.validator.Validate(ctx,
		auth.ValidateMulticastGroupQueueAccess(auth.Read, mgID)); err != nil {
		return nil, grpc.Errorf(codes.Unauthenticated, "authentication failed: %s", err)
	}

	queueItems, err := multicast.ListQueue(ctx, storage.DB(), mgID)
	if err != nil {
		return nil, helpers.ErrToRPCError(err)
	}

	var resp pb.ListMulticastGroupQueueItemsResponse
	for i := range queueItems {
		resp.MulticastQueueItems = append(resp.MulticastQueueItems, &queueItems[i])
	}

	return &resp, nil
}
