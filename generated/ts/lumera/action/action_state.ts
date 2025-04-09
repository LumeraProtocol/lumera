// Code generated by protoc-gen-ts_proto. DO NOT EDIT.
// versions:
//   protoc-gen-ts_proto  v2.6.1
//   protoc               unknown
// source: lumera/action/action_state.proto

/* eslint-disable */

export const protobufPackage = "lumera.action";

export enum ActionState {
  ACTION_STATE_UNSPECIFIED = 0,
  ACTION_STATE_PENDING = 1,
  ACTION_STATE_PROCESSING = 2,
  ACTION_STATE_DONE = 3,
  ACTION_STATE_APPROVED = 4,
  ACTION_STATE_REJECTED = 5,
  ACTION_STATE_FAILED = 6,
  ACTION_STATE_EXPIRED = 7,
  UNRECOGNIZED = -1,
}

export function actionStateFromJSON(object: any): ActionState {
  switch (object) {
    case 0:
    case "ACTION_STATE_UNSPECIFIED":
      return ActionState.ACTION_STATE_UNSPECIFIED;
    case 1:
    case "ACTION_STATE_PENDING":
      return ActionState.ACTION_STATE_PENDING;
    case 2:
    case "ACTION_STATE_PROCESSING":
      return ActionState.ACTION_STATE_PROCESSING;
    case 3:
    case "ACTION_STATE_DONE":
      return ActionState.ACTION_STATE_DONE;
    case 4:
    case "ACTION_STATE_APPROVED":
      return ActionState.ACTION_STATE_APPROVED;
    case 5:
    case "ACTION_STATE_REJECTED":
      return ActionState.ACTION_STATE_REJECTED;
    case 6:
    case "ACTION_STATE_FAILED":
      return ActionState.ACTION_STATE_FAILED;
    case 7:
    case "ACTION_STATE_EXPIRED":
      return ActionState.ACTION_STATE_EXPIRED;
    case -1:
    case "UNRECOGNIZED":
    default:
      return ActionState.UNRECOGNIZED;
  }
}

export function actionStateToJSON(object: ActionState): string {
  switch (object) {
    case ActionState.ACTION_STATE_UNSPECIFIED:
      return "ACTION_STATE_UNSPECIFIED";
    case ActionState.ACTION_STATE_PENDING:
      return "ACTION_STATE_PENDING";
    case ActionState.ACTION_STATE_PROCESSING:
      return "ACTION_STATE_PROCESSING";
    case ActionState.ACTION_STATE_DONE:
      return "ACTION_STATE_DONE";
    case ActionState.ACTION_STATE_APPROVED:
      return "ACTION_STATE_APPROVED";
    case ActionState.ACTION_STATE_REJECTED:
      return "ACTION_STATE_REJECTED";
    case ActionState.ACTION_STATE_FAILED:
      return "ACTION_STATE_FAILED";
    case ActionState.ACTION_STATE_EXPIRED:
      return "ACTION_STATE_EXPIRED";
    case ActionState.UNRECOGNIZED:
    default:
      return "UNRECOGNIZED";
  }
}
