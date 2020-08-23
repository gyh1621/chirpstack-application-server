import React, {Component} from "react";
import {withRouter} from 'react-router-dom';

import {withStyles} from "@material-ui/core/styles";
import Grid from '@material-ui/core/Grid';
import Card from '@material-ui/core/Card';
import CardContent from "@material-ui/core/CardContent";

import TitleBar from "../../components/TitleBar";
import TitleBarTitle from "../../components/TitleBarTitle";
import FUOTADeploymentStore from "../../stores/FUOTADeploymentStore";
import FUOTADeploymentForm from "./FUOTADeploymentForm";


const styles = {
  card: {
    overflow: "visible",
  },
};


class CreateFUOTADeploymentForGroup extends Component {
  constructor() {
    super();
    this.state = {};
    this.onSubmit = this.onSubmit.bind(this);
  }

  onSubmit(fuotaDeployment) {
    FUOTADeploymentStore.createForGroup(fuotaDeployment.mcGroupID, fuotaDeployment, resp => {
      this.props.history.push(`/organizations/${this.props.match.params.organizationID}/fuota-deployments`);
    });
  }

  render() {
    return (
      <Grid container spacing={4}>
        <TitleBar>
          <TitleBarTitle title="FUOTA"
                         to={`/organizations/${this.props.match.params.organizationID}/fuota-deployments`}/>
          <TitleBarTitle title="/"/>
          <TitleBarTitle title="Create update job for multicast group"/>
        </TitleBar>

        <Grid item xs={12}>
          <Card className={this.props.classes.card}>
            <CardContent>
              <FUOTADeploymentForm
                type={"group"}
                submitLabel="Create FUOTA deployment"
                onSubmit={this.onSubmit}
                props={this.props}
              />
            </CardContent>
          </Card>
        </Grid>
      </Grid>
    );
  }
}

export default withStyles(styles)(withRouter(CreateFUOTADeploymentForGroup));

