import React, {Component} from "react";
import {withRouter} from 'react-router-dom';

import {withStyles} from "@material-ui/core/styles";
import Grid from '@material-ui/core/Grid';
import Card from '@material-ui/core/Card';
import CardContent from "@material-ui/core/CardContent";
import TextField from "@material-ui/core/TextField";

import Form from "../../components/Form";
import FormComponent from "../../classes/FormComponent";
import MulticastGroupStore from "../../stores/MulticastGroupStore";


const styles = {
  card: {
    overflow: "visible",
  },
};

class SendMulticastMessageForm extends FormComponent {

  render() {
    if (this.state.object === undefined) {
      return null;
    }

    return (
      <Form
        submitLabel={this.props.submitLabel}
        onSubmit={this.onSubmit}
      >
        <TextField
          id="message"
          label="Multicast Message"
          margin="normal"
          fullWidth
          multiline
          rows={2}
          required
          onChange={this.onChange}
          variant="outlined"
        />
      </Form>
    );
  }
}

class SendMulticastMessage extends Component {
  constructor() {
    super();
    this.onSubmit = this.onSubmit.bind(this);
  }

  onSubmit(message) {
    const multicastGroupQueueItem = {
      multicastQueueItem: {
        data: btoa(message.message),
        fPort: 200,
      },
    };
    MulticastGroupStore.sendMessage(this.props.multicastGroup, multicastGroupQueueItem, resp => {
      this.props.history.push(`/organizations/${this.props.match.params.organizationID}/multicast-groups/${this.props.match.params.multicastGroupID}`);
    });
  }

  render() {
    if (this.props.multicastGroup === undefined) {
      return null;
    }

    return (
      <Grid container spacing={4}>
        <Grid item xs={12}>
          <Card className={this.props.classes.card}>
            <CardContent>
              <SendMulticastMessageForm
                submitLabel="Send Message"
                onSubmit={this.onSubmit}
              />
            </CardContent>
          </Card>
        </Grid>
      </Grid>
    );
  }
}

export default withStyles(styles)(withRouter(SendMulticastMessage));
