// Code generated by protoc-gen-ts_proto. DO NOT EDIT.
// versions:
//   protoc-gen-ts_proto  v2.6.1
//   protoc               unknown
// source: lumera/action/action_type.proto

/* eslint-disable */

export const protobufPackage = "lumera.action";

export enum ActionType {
  ACTION_TYPE_UNSPECIFIED = 0,
  ACTION_TYPE_SENSE = 1,
  ACTION_TYPE_CASCADE = 2,
  UNRECOGNIZED = -1,
}

export function actionTypeFromJSON(object: any): ActionType {
  switch (object) {
    case 0:
    case "ACTION_TYPE_UNSPECIFIED":
      return ActionType.ACTION_TYPE_UNSPECIFIED;
    case 1:
    case "ACTION_TYPE_SENSE":
      return ActionType.ACTION_TYPE_SENSE;
    case 2:
    case "ACTION_TYPE_CASCADE":
      return ActionType.ACTION_TYPE_CASCADE;
    case -1:
    case "UNRECOGNIZED":
    default:
      return ActionType.UNRECOGNIZED;
  }
}

export function actionTypeToJSON(object: ActionType): string {
  switch (object) {
    case ActionType.ACTION_TYPE_UNSPECIFIED:
      return "ACTION_TYPE_UNSPECIFIED";
    case ActionType.ACTION_TYPE_SENSE:
      return "ACTION_TYPE_SENSE";
    case ActionType.ACTION_TYPE_CASCADE:
      return "ACTION_TYPE_CASCADE";
    case ActionType.UNRECOGNIZED:
    default:
      return "UNRECOGNIZED";
  }
}
