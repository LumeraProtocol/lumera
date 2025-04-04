// Code generated by protoc-gen-ts_proto. DO NOT EDIT.
// versions:
//   protoc-gen-ts_proto  v2.6.1
//   protoc               unknown
// source: lumera/claim/tx.proto

/* eslint-disable */
import { BinaryReader, BinaryWriter } from "@bufbuild/protobuf/wire";
import { Params } from "./params";

export const protobufPackage = "lumera.claim";

/**
 * MsgUpdateParams is the Msg/UpdateParams request type.
 * MsgUpdateParams is the Msg/UpdateParams request type.
 */
export interface MsgUpdateParams {
  /** authority is the address that controls the module (defaults to x/gov unless overwritten). */
  authority: string;
  /**
   * params defines the x/claim parameters to update.
   * NOTE: All parameters must be supplied.
   */
  params: Params | undefined;
}

/**
 * MsgUpdateParamsResponse defines the response structure for executing a
 * MsgUpdateParams message.
 */
export interface MsgUpdateParamsResponse {
}

export interface MsgClaim {
  creator: string;
  oldAddress: string;
  newAddress: string;
  pubKey: string;
  signature: string;
}

export interface MsgClaimResponse {
}

function createBaseMsgUpdateParams(): MsgUpdateParams {
  return { authority: "", params: undefined };
}

export const MsgUpdateParams: MessageFns<MsgUpdateParams> = {
  encode(message: MsgUpdateParams, writer: BinaryWriter = new BinaryWriter()): BinaryWriter {
    if (message.authority !== "") {
      writer.uint32(10).string(message.authority);
    }
    if (message.params !== undefined) {
      Params.encode(message.params, writer.uint32(18).fork()).join();
    }
    return writer;
  },

  decode(input: BinaryReader | Uint8Array, length?: number): MsgUpdateParams {
    const reader = input instanceof BinaryReader ? input : new BinaryReader(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseMsgUpdateParams();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1: {
          if (tag !== 10) {
            break;
          }

          message.authority = reader.string();
          continue;
        }
        case 2: {
          if (tag !== 18) {
            break;
          }

          message.params = Params.decode(reader, reader.uint32());
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

  fromJSON(object: any): MsgUpdateParams {
    return {
      authority: isSet(object.authority) ? globalThis.String(object.authority) : "",
      params: isSet(object.params) ? Params.fromJSON(object.params) : undefined,
    };
  },

  toJSON(message: MsgUpdateParams): unknown {
    const obj: any = {};
    if (message.authority !== "") {
      obj.authority = message.authority;
    }
    if (message.params !== undefined) {
      obj.params = Params.toJSON(message.params);
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<MsgUpdateParams>, I>>(base?: I): MsgUpdateParams {
    return MsgUpdateParams.fromPartial(base ?? ({} as any));
  },
  fromPartial<I extends Exact<DeepPartial<MsgUpdateParams>, I>>(object: I): MsgUpdateParams {
    const message = createBaseMsgUpdateParams();
    message.authority = object.authority ?? "";
    message.params = (object.params !== undefined && object.params !== null)
      ? Params.fromPartial(object.params)
      : undefined;
    return message;
  },
};

function createBaseMsgUpdateParamsResponse(): MsgUpdateParamsResponse {
  return {};
}

export const MsgUpdateParamsResponse: MessageFns<MsgUpdateParamsResponse> = {
  encode(_: MsgUpdateParamsResponse, writer: BinaryWriter = new BinaryWriter()): BinaryWriter {
    return writer;
  },

  decode(input: BinaryReader | Uint8Array, length?: number): MsgUpdateParamsResponse {
    const reader = input instanceof BinaryReader ? input : new BinaryReader(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseMsgUpdateParamsResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skip(tag & 7);
    }
    return message;
  },

  fromJSON(_: any): MsgUpdateParamsResponse {
    return {};
  },

  toJSON(_: MsgUpdateParamsResponse): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<MsgUpdateParamsResponse>, I>>(base?: I): MsgUpdateParamsResponse {
    return MsgUpdateParamsResponse.fromPartial(base ?? ({} as any));
  },
  fromPartial<I extends Exact<DeepPartial<MsgUpdateParamsResponse>, I>>(_: I): MsgUpdateParamsResponse {
    const message = createBaseMsgUpdateParamsResponse();
    return message;
  },
};

function createBaseMsgClaim(): MsgClaim {
  return { creator: "", oldAddress: "", newAddress: "", pubKey: "", signature: "" };
}

export const MsgClaim: MessageFns<MsgClaim> = {
  encode(message: MsgClaim, writer: BinaryWriter = new BinaryWriter()): BinaryWriter {
    if (message.creator !== "") {
      writer.uint32(10).string(message.creator);
    }
    if (message.oldAddress !== "") {
      writer.uint32(18).string(message.oldAddress);
    }
    if (message.newAddress !== "") {
      writer.uint32(26).string(message.newAddress);
    }
    if (message.pubKey !== "") {
      writer.uint32(34).string(message.pubKey);
    }
    if (message.signature !== "") {
      writer.uint32(42).string(message.signature);
    }
    return writer;
  },

  decode(input: BinaryReader | Uint8Array, length?: number): MsgClaim {
    const reader = input instanceof BinaryReader ? input : new BinaryReader(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseMsgClaim();
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

          message.oldAddress = reader.string();
          continue;
        }
        case 3: {
          if (tag !== 26) {
            break;
          }

          message.newAddress = reader.string();
          continue;
        }
        case 4: {
          if (tag !== 34) {
            break;
          }

          message.pubKey = reader.string();
          continue;
        }
        case 5: {
          if (tag !== 42) {
            break;
          }

          message.signature = reader.string();
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

  fromJSON(object: any): MsgClaim {
    return {
      creator: isSet(object.creator) ? globalThis.String(object.creator) : "",
      oldAddress: isSet(object.oldAddress) ? globalThis.String(object.oldAddress) : "",
      newAddress: isSet(object.newAddress) ? globalThis.String(object.newAddress) : "",
      pubKey: isSet(object.pubKey) ? globalThis.String(object.pubKey) : "",
      signature: isSet(object.signature) ? globalThis.String(object.signature) : "",
    };
  },

  toJSON(message: MsgClaim): unknown {
    const obj: any = {};
    if (message.creator !== "") {
      obj.creator = message.creator;
    }
    if (message.oldAddress !== "") {
      obj.oldAddress = message.oldAddress;
    }
    if (message.newAddress !== "") {
      obj.newAddress = message.newAddress;
    }
    if (message.pubKey !== "") {
      obj.pubKey = message.pubKey;
    }
    if (message.signature !== "") {
      obj.signature = message.signature;
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<MsgClaim>, I>>(base?: I): MsgClaim {
    return MsgClaim.fromPartial(base ?? ({} as any));
  },
  fromPartial<I extends Exact<DeepPartial<MsgClaim>, I>>(object: I): MsgClaim {
    const message = createBaseMsgClaim();
    message.creator = object.creator ?? "";
    message.oldAddress = object.oldAddress ?? "";
    message.newAddress = object.newAddress ?? "";
    message.pubKey = object.pubKey ?? "";
    message.signature = object.signature ?? "";
    return message;
  },
};

function createBaseMsgClaimResponse(): MsgClaimResponse {
  return {};
}

export const MsgClaimResponse: MessageFns<MsgClaimResponse> = {
  encode(_: MsgClaimResponse, writer: BinaryWriter = new BinaryWriter()): BinaryWriter {
    return writer;
  },

  decode(input: BinaryReader | Uint8Array, length?: number): MsgClaimResponse {
    const reader = input instanceof BinaryReader ? input : new BinaryReader(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseMsgClaimResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skip(tag & 7);
    }
    return message;
  },

  fromJSON(_: any): MsgClaimResponse {
    return {};
  },

  toJSON(_: MsgClaimResponse): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<MsgClaimResponse>, I>>(base?: I): MsgClaimResponse {
    return MsgClaimResponse.fromPartial(base ?? ({} as any));
  },
  fromPartial<I extends Exact<DeepPartial<MsgClaimResponse>, I>>(_: I): MsgClaimResponse {
    const message = createBaseMsgClaimResponse();
    return message;
  },
};

/** Msg defines the Msg service. */
export interface Msg {
  /**
   * UpdateParams defines a (governance) operation for updating the module
   * parameters. The authority defaults to the x/gov module account.
   */
  UpdateParams(request: MsgUpdateParams): Promise<MsgUpdateParamsResponse>;
  Claim(request: MsgClaim): Promise<MsgClaimResponse>;
}

export const MsgServiceName = "lumera.claim.Msg";
export class MsgClientImpl implements Msg {
  private readonly rpc: Rpc;
  private readonly service: string;
  constructor(rpc: Rpc, opts?: { service?: string }) {
    this.service = opts?.service || MsgServiceName;
    this.rpc = rpc;
    this.UpdateParams = this.UpdateParams.bind(this);
    this.Claim = this.Claim.bind(this);
  }
  UpdateParams(request: MsgUpdateParams): Promise<MsgUpdateParamsResponse> {
    const data = MsgUpdateParams.encode(request).finish();
    const promise = this.rpc.request(this.service, "UpdateParams", data);
    return promise.then((data) => MsgUpdateParamsResponse.decode(new BinaryReader(data)));
  }

  Claim(request: MsgClaim): Promise<MsgClaimResponse> {
    const data = MsgClaim.encode(request).finish();
    const promise = this.rpc.request(this.service, "Claim", data);
    return promise.then((data) => MsgClaimResponse.decode(new BinaryReader(data)));
  }
}

interface Rpc {
  request(service: string, method: string, data: Uint8Array): Promise<Uint8Array>;
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
