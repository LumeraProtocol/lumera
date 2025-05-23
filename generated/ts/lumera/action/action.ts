// Code generated by protoc-gen-ts_proto. DO NOT EDIT.
// versions:
//   protoc-gen-ts_proto  v2.6.1
//   protoc               unknown
// source: lumera/action/action.proto

/* eslint-disable */
import { BinaryReader, BinaryWriter } from "@bufbuild/protobuf/wire";
import { ActionState, actionStateFromJSON, actionStateToJSON } from "./action_state";
import { ActionType, actionTypeFromJSON, actionTypeToJSON } from "./action_type";

export const protobufPackage = "lumera.action";

export interface Action {
  creator: string;
  actionID: string;
  actionType: ActionType;
  metadata: Uint8Array;
  price: string;
  expirationTime: number;
  state: ActionState;
  blockHeight: number;
  superNodes: string[];
}

function createBaseAction(): Action {
  return {
    creator: "",
    actionID: "",
    actionType: 0,
    metadata: new Uint8Array(0),
    price: "",
    expirationTime: 0,
    state: 0,
    blockHeight: 0,
    superNodes: [],
  };
}

export const Action: MessageFns<Action> = {
  encode(message: Action, writer: BinaryWriter = new BinaryWriter()): BinaryWriter {
    if (message.creator !== "") {
      writer.uint32(10).string(message.creator);
    }
    if (message.actionID !== "") {
      writer.uint32(18).string(message.actionID);
    }
    if (message.actionType !== 0) {
      writer.uint32(24).int32(message.actionType);
    }
    if (message.metadata.length !== 0) {
      writer.uint32(34).bytes(message.metadata);
    }
    if (message.price !== "") {
      writer.uint32(42).string(message.price);
    }
    if (message.expirationTime !== 0) {
      writer.uint32(48).int64(message.expirationTime);
    }
    if (message.state !== 0) {
      writer.uint32(56).int32(message.state);
    }
    if (message.blockHeight !== 0) {
      writer.uint32(64).int64(message.blockHeight);
    }
    for (const v of message.superNodes) {
      writer.uint32(74).string(v!);
    }
    return writer;
  },

  decode(input: BinaryReader | Uint8Array, length?: number): Action {
    const reader = input instanceof BinaryReader ? input : new BinaryReader(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAction();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1: {
          if (tag !== 10) {
            break;
          }

          message.creator = reader.string();
          continue;
        }
        case 2: {
          if (tag !== 18) {
            break;
          }

          message.actionID = reader.string();
          continue;
        }
        case 3: {
          if (tag !== 24) {
            break;
          }

          message.actionType = reader.int32() as any;
          continue;
        }
        case 4: {
          if (tag !== 34) {
            break;
          }

          message.metadata = reader.bytes();
          continue;
        }
        case 5: {
          if (tag !== 42) {
            break;
          }

          message.price = reader.string();
          continue;
        }
        case 6: {
          if (tag !== 48) {
            break;
          }

          message.expirationTime = longToNumber(reader.int64());
          continue;
        }
        case 7: {
          if (tag !== 56) {
            break;
          }

          message.state = reader.int32() as any;
          continue;
        }
        case 8: {
          if (tag !== 64) {
            break;
          }

          message.blockHeight = longToNumber(reader.int64());
          continue;
        }
        case 9: {
          if (tag !== 74) {
            break;
          }

          message.superNodes.push(reader.string());
          continue;
        }
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skip(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Action {
    return {
      creator: isSet(object.creator) ? globalThis.String(object.creator) : "",
      actionID: isSet(object.actionID) ? globalThis.String(object.actionID) : "",
      actionType: isSet(object.actionType) ? actionTypeFromJSON(object.actionType) : 0,
      metadata: isSet(object.metadata) ? bytesFromBase64(object.metadata) : new Uint8Array(0),
      price: isSet(object.price) ? globalThis.String(object.price) : "",
      expirationTime: isSet(object.expirationTime) ? globalThis.Number(object.expirationTime) : 0,
      state: isSet(object.state) ? actionStateFromJSON(object.state) : 0,
      blockHeight: isSet(object.blockHeight) ? globalThis.Number(object.blockHeight) : 0,
      superNodes: globalThis.Array.isArray(object?.superNodes)
        ? object.superNodes.map((e: any) => globalThis.String(e))
        : [],
    };
  },

  toJSON(message: Action): unknown {
    const obj: any = {};
    if (message.creator !== "") {
      obj.creator = message.creator;
    }
    if (message.actionID !== "") {
      obj.actionID = message.actionID;
    }
    if (message.actionType !== 0) {
      obj.actionType = actionTypeToJSON(message.actionType);
    }
    if (message.metadata.length !== 0) {
      obj.metadata = base64FromBytes(message.metadata);
    }
    if (message.price !== "") {
      obj.price = message.price;
    }
    if (message.expirationTime !== 0) {
      obj.expirationTime = Math.round(message.expirationTime);
    }
    if (message.state !== 0) {
      obj.state = actionStateToJSON(message.state);
    }
    if (message.blockHeight !== 0) {
      obj.blockHeight = Math.round(message.blockHeight);
    }
    if (message.superNodes?.length) {
      obj.superNodes = message.superNodes;
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<Action>, I>>(base?: I): Action {
    return Action.fromPartial(base ?? ({} as any));
  },
  fromPartial<I extends Exact<DeepPartial<Action>, I>>(object: I): Action {
    const message = createBaseAction();
    message.creator = object.creator ?? "";
    message.actionID = object.actionID ?? "";
    message.actionType = object.actionType ?? 0;
    message.metadata = object.metadata ?? new Uint8Array(0);
    message.price = object.price ?? "";
    message.expirationTime = object.expirationTime ?? 0;
    message.state = object.state ?? 0;
    message.blockHeight = object.blockHeight ?? 0;
    message.superNodes = object.superNodes?.map((e) => e) || [];
    return message;
  },
};

function bytesFromBase64(b64: string): Uint8Array {
  if ((globalThis as any).Buffer) {
    return Uint8Array.from(globalThis.Buffer.from(b64, "base64"));
  } else {
    const bin = globalThis.atob(b64);
    const arr = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; ++i) {
      arr[i] = bin.charCodeAt(i);
    }
    return arr;
  }
}

function base64FromBytes(arr: Uint8Array): string {
  if ((globalThis as any).Buffer) {
    return globalThis.Buffer.from(arr).toString("base64");
  } else {
    const bin: string[] = [];
    arr.forEach((byte) => {
      bin.push(globalThis.String.fromCharCode(byte));
    });
    return globalThis.btoa(bin.join(""));
  }
}

type Builtin = Date | Function | Uint8Array | string | number | boolean | undefined;

export type DeepPartial<T> = T extends Builtin ? T
  : T extends globalThis.Array<infer U> ? globalThis.Array<DeepPartial<U>>
  : T extends ReadonlyArray<infer U> ? ReadonlyArray<DeepPartial<U>>
  : T extends {} ? { [K in keyof T]?: DeepPartial<T[K]> }
  : Partial<T>;

type KeysOfUnion<T> = T extends T ? keyof T : never;
export type Exact<P, I extends P> = P extends Builtin ? P
  : P & { [K in keyof P]: Exact<P[K], I[K]> } & { [K in Exclude<keyof I, KeysOfUnion<P>>]: never };

function longToNumber(int64: { toString(): string }): number {
  const num = globalThis.Number(int64.toString());
  if (num > globalThis.Number.MAX_SAFE_INTEGER) {
    throw new globalThis.Error("Value is larger than Number.MAX_SAFE_INTEGER");
  }
  if (num < globalThis.Number.MIN_SAFE_INTEGER) {
    throw new globalThis.Error("Value is smaller than Number.MIN_SAFE_INTEGER");
  }
  return num;
}

function isSet(value: any): boolean {
  return value !== null && value !== undefined;
}

export interface MessageFns<T> {
  encode(message: T, writer?: BinaryWriter): BinaryWriter;
  decode(input: BinaryReader | Uint8Array, length?: number): T;
  fromJSON(object: any): T;
  toJSON(message: T): unknown;
  create<I extends Exact<DeepPartial<T>, I>>(base?: I): T;
  fromPartial<I extends Exact<DeepPartial<T>, I>>(object: I): T;
}
