import React, {Component} from "react";

import {withStyles} from "@material-ui/core/styles";
import Grid from "@material-ui/core/Grid";
import TableCell from "@material-ui/core/TableCell";
import TableRow from "@material-ui/core/TableRow";
import Plus from "mdi-material-ui/Plus";

import moment from "moment";

import TitleBar from "../../components/TitleBar";
import TitleBarTitle from "../../components/TitleBarTitle";
import TableCellLink from "../../components/TableCellLink";
import TitleBarButton from "../../components/TitleBarButton";
import DataTable from "../../components/DataTable";
import FUOTADeploymentStore from "../../stores/FUOTADeploymentStore";
import theme from "../../theme";
import Admin from "../../components/Admin";


const styles = {
  buttons: {
    textAlign: "right",
  },
  button: {
    marginLeft: 2 * theme.spacing(1),
  },
  icon: {
    marginRight: theme.spacing(1),
  },
};


class ListFUOTADeployments extends Component {
  constructor() {
    super();

    this.state = {};

    this.getPage = this.getPage.bind(this);
    this.getRow = this.getRow.bind(this);
  }

  getPage(limit, offset, callbackFunc) {
    FUOTADeploymentStore.list({
      limit: limit,
      offset: offset,
    }, callbackFunc);
  }

  getRow(obj) {
    const createdAt = moment(obj.createdAt).format('lll');
    const updatedAt = moment(obj.updatedAt).format('lll');

    return (
      <TableRow
        key={obj.id}
        hover
      >
        <TableCellLink
          to={`/organizations/${this.props.match.params.organizationID}/fuota-deployments/${obj.id}`}>{obj.name}</TableCellLink>
        <TableCell>{obj.type}</TableCell>
        <TableCell>{createdAt}</TableCell>
        <TableCell>{updatedAt}</TableCell>
        <TableCell>{obj.state}</TableCell>
      </TableRow>
    );
  }

  render() {
    return (
      <Grid container spacing={4}>
        <TitleBar
          buttons={
            <Admin organizationID={this.props.match.params.organizationID}>
              <TitleBarButton
                label="Create For Device"
                icon={<Plus/>}
                to={`/organizations/${this.props.match.params.organizationID}/fuota-deployments/create-for-device`}
              />
              <TitleBarButton
                label="Create For Multicast Group"
                icon={<Plus/>}
                to={`/organizations/${this.props.match.params.organizationID}/fuota-deployments/create-for-group`}
              />
            </Admin>
          }
        >
          <TitleBarTitle title="FUOTA"/>
        </TitleBar>
        <Grid item xs={12}>
          <DataTable
            header={
              <TableRow>
                <TableCell>Name</TableCell>
                <TableCell>Type</TableCell>
                <TableCell>Created at</TableCell>
                <TableCell>Updated at</TableCell>
                <TableCell>State</TableCell>
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

export default withStyles(styles)(ListFUOTADeployments);
