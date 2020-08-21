import React, {Component} from "react";
import {withRouter} from 'react-router-dom';

import moment from "moment";
import {withStyles} from "@material-ui/core/styles";
import Grid from '@material-ui/core/Grid';
import Card from '@material-ui/core/Card';
import CardContent from "@material-ui/core/CardContent";
import Checkbox from '@material-ui/core/Checkbox';
import FormControlLabel from '@material-ui/core/FormControlLabel';
import TableCell from "@material-ui/core/TableCell";
import TableRow from "@material-ui/core/TableRow";
import Button from '@material-ui/core/Button';

import DeviceStore from "../../stores/DeviceStore";
import TableCellLink from "../../components/TableCellLink";
import DataTable from "../../components/DataTable";
import TitleBar from "../../components/TitleBar";
import TitleBarTitle from "../../components/TitleBarTitle";
import MulticastGroupStore from "../../stores/MulticastGroupStore";
import AutocompleteSelect from "../../components/AutocompleteSelect";
import ApplicationStore from "../../stores/ApplicationStore";


const styles = {
  card: {
    overflow: "visible",
  },
  formLabel: {
    fontSize: 12,
  },
};

class SelectApplication extends Component {
  constructor() {
    super();

    this.getApplicationOptions = this.getApplicationOptions.bind(this);
  }

  getApplicationOptions(search, callbackFunc) {
    ApplicationStore.list(search, this.props.organizationID, 10, 0, resp => {
      const options = resp.result
        .filter((a, i) => {
          return a.serviceProfileID === this.props.serviceProfileID
        })
        .map((a, i) => {
          return {label: `${a.name} (${a.id})`, value: a.id}
        });
      callbackFunc(options);
    });
  }

  render() {
    return (
      <AutocompleteSelect
        id="applicationID"
        label="Select device"
        onChange={this.props.onChange}
        getOptions={this.getApplicationOptions}
        margin="none"
      />
    );
  }
}

SelectApplication = withStyles(styles)(SelectApplication)

class ListApplicationDevices extends Component {
  constructor() {
    super();

    this.state = {
      joinedStatus: new Map(),
    }

    this.getPage = this.getPage.bind(this);
    this.getRow = this.getRow.bind(this);
  }

  shouldComponentUpdate(nextProps, nextState, nextContext) {
    return (this.props.applicationID !== nextProps.applicationID
      || this.props.clickAllTimes !== nextProps.clickAllTimes
      || this.state !== nextState);
  }

  checkJoined(devEUI, callbackFunc) {
    if (this.state.joinedStatus.has(devEUI)) return;
    DeviceStore.list({multicastGroupID: this.props.match.params.multicastGroupID, search: devEUI, limit: 1}, resp => {
      this.state.joinedStatus.set(devEUI, resp.totalCount === "1");
      callbackFunc(resp);
    });
  }

  getPage(limit, offset, callbackFunc) {
    DeviceStore.list({
      applicationID: this.props.applicationID,
      limit: limit,
      offset: offset,
    }, resp => {
      let number = resp.result.length;
      resp.result.forEach(
        device => {
          this.checkJoined(device.devEUI, resp => {
            number--;
            if (number <= 0) {
              this.setState({
                joinedStatus: this.state.joinedStatus,
              });
            }
          });
        }
      );
      callbackFunc(resp);
    });
  }

  getRow(obj) {
    let lastseen = "n/a";
    if (obj.lastSeenAt !== undefined && obj.lastSeenAt !== null) {
      lastseen = moment(obj.lastSeenAt).fromNow();
    }

    let status = "";
    if (this.state.joinedStatus.get(obj.devEUI)) {
      status = "✓";
    }

    return (
      <TableRow
        key={obj.devEUI}
        hover
      >
        <TableCell width="5%">
          <Checkbox
            key={this.props.clickAllTimes.toString() + obj.devEUI + this.state.joinedStatus.get(obj.devEUI)}
            id={obj.devEUI}
            onClick={this.props.onCheck}
            defaultChecked={this.state.joinedStatus.get(obj.devEUI) || this.props.getCheckStatus(obj.devEUI)}
            disabled={this.state.joinedStatus.get(obj.devEUI)}
          />
        </TableCell>
        <TableCell width="5%">{status}</TableCell>
        <TableCell width="10%">{lastseen}</TableCell>
        <TableCellLink width="25%"
                       to={`/organizations/${this.props.match.params.organizationID}/applications/${obj.applicationID}/devices/${obj.devEUI}`}>{obj.name}</TableCellLink>
        <TableCell width="25%">{obj.devEUI}</TableCell>
        <TableCellLink witdth="25%"
                       to={`/organizations/${this.props.match.params.organizationID}/device-profiles/${obj.deviceProfileID}`}>{obj.deviceProfileName}</TableCellLink>
      </TableRow>
    );
  }

  render() {
    return (
      <Grid container spacing={4}>
        <Grid item xs={12}>
          <DataTable
            header={
              <TableRow>
                <TableCell width="5%"/>
                <TableCell width="5%">Joined</TableCell>
                <TableCell width="10%">Last Seen</TableCell>
                <TableCell width="25%">Device name</TableCell>
                <TableCell width="25%">Device EUI</TableCell>
                <TableCell width="25%">Device profile</TableCell>
              </TableRow>
            }
            getPage={this.getPage}
            getRow={this.getRow}
          />
        </Grid>
      </Grid>
    );
  }
}


class AddDeviceToMulticastGroup extends Component {
  constructor() {
    super();
    this.state = {
      selectedAppID: -1,
      selectedDevices: new Set(),
      selectedAll: false,
      // keep track of click times of "select all", for force updating checkboxes
      clickAllTimes: 0,
    };
    this.onSubmit = this.onSubmit.bind(this);
  }

  componentDidMount() {
    MulticastGroupStore.get(this.props.match.params.multicastGroupID, resp => {
      this.setState({
        multicastGroup: resp.multicastGroup,
      });
    });
  }

  onSubmit(device) {
    const {selectedAppID, selectedAll, selectedDevices} = this.state;
    if (selectedAll) {
      MulticastGroupStore.addApplicationDevice(
        this.props.match.params.multicastGroupID,
        selectedAppID,
        Array.from(selectedDevices),
        resp => {
          this.props.history.push(`/organizations/${this.props.match.params.organizationID}/multicast-groups/${this.props.match.params.multicastGroupID}`);
        }
      )
    } else {
      MulticastGroupStore.addDevice(this.props.match.params.multicastGroupID, Array.from(selectedDevices), resp => {
        this.props.history.push(`/organizations/${this.props.match.params.organizationID}/multicast-groups/${this.props.match.params.multicastGroupID}`);
      })
    }
  }

  selectApplication(e) {
    this.setState({
      selectedAppID: e.target.value,
      selectedDevices: new Set(),
      selectedAll: false,
    });
  }

  selectDevice(e) {
    const devEUI = e.target.id;
    const selectedDevices = this.state.selectedDevices;
    if (e.target.checked) {
      if (this.state.selectedAll) {
        selectedDevices.delete(devEUI);
      } else {
        selectedDevices.add(devEUI);
      }
    } else {
      if (this.state.selectedAll) {
        selectedDevices.add(devEUI);
      } else {
        selectedDevices.delete(devEUI);
      }
    }
    this.setState({selectedDevices: selectedDevices})
  }

  getCheckStatus(devEUI) {
    if (this.state.selectedAll) {
      if (this.state.selectedDevices.size === 0) {
        return true;
      } else {
        return !this.state.selectedDevices.has(devEUI);
      }
    } else {
      return this.state.selectedDevices.has(devEUI);
    }
  }

  toggleSelectAllDevices(e) {
    if (e.target.checked || e.target.indeterminate) {
      this.setState({
        selectedAll: true,
        selectedDevices: new Set(),
        clickAllTimes: this.state.clickAllTimes + 1,
      });
    } else {
      this.setState({
        selectedAll: false,
        selectedDevices: new Set(),
        clickAllTimes: this.state.clickAllTimes + 1,
      });
    }
  }

  render() {
    if (this.state.multicastGroup === undefined) {
      return null;
    }

    return (
      <Grid container spacing={4}>
        <TitleBar>
          <TitleBarTitle title="Multicast-groups"
                         to={`/organizations/${this.props.match.params.organizationID}/multicast-groups`}/>
          <TitleBarTitle title="/"/>
          <TitleBarTitle title={this.state.multicastGroup.name}
                         to={`/organizations/${this.props.match.params.organizationID}/multicast-groups/${this.state.multicastGroup.id}`}/>
          <TitleBarTitle title="/"/>
          <TitleBarTitle title="Add device"/>
        </TitleBar>

        <Grid item xs={12}>
          <Card className={this.props.classes.card}>
            <CardContent>
              <Grid container spacing={4} alignItems="flex-end">
                <Grid item xs={4}>
                  <SelectApplication
                    defaultApplicationID={this.state.applicationID || ""}
                    organizationID={this.props.organizationID}
                    serviceProfileID={this.state.multicastGroup.serviceProfileID}
                    onChange={e => this.selectApplication(e)}
                  />
                </Grid>

                <Grid item xs={8}>
                  <FormControlLabel
                    control={<Checkbox
                      name="selectAll"
                      indeterminate={this.state.selectedAll && this.state.selectedDevices.size !== 0}
                      checked={this.state.selectedAll && this.state.selectedDevices.size === 0}
                      onClick={e => {
                        this.toggleSelectAllDevices(e)
                      }}
                      disabled={this.state.selectedAppID === -1}
                    />}
                    label="select all of this application"
                  />
                </Grid>

                <Grid item xs={12}>
                  <ListApplicationDevices
                    applicationID={this.state.selectedAppID}
                    clickAllTimes={this.state.clickAllTimes}
                    onCheck={e => {
                      this.selectDevice(e)
                    }}
                    getCheckStatus={devEUI => {
                      return this.getCheckStatus(devEUI)
                    }}
                    {...this.props}
                  />
                </Grid>

                <Grid item xs={10}/>
                <Grid item xs={2}>
                  <Button
                    variant="outlined"
                    color="primary"
                    onClick={this.onSubmit}
                    disabled={!this.state.selectedAppID && this.state.selectedDevices.size === 0}
                  >
                    ADD DEVICES
                  </Button>
                </Grid>
              </Grid>
            </CardContent>
          </Card>
        </Grid>
      </Grid>
    );
  }
}

export default withStyles(styles)(withRouter(AddDeviceToMulticastGroup))
