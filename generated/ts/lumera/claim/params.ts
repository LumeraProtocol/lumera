// Code generated by protoc-gen-ts_proto. DO NOT EDIT.
// versions:
//   protoc-gen-ts_proto  v2.6.1
//   protoc               unknown
// source: lumera/claim/params.proto

/* eslint-disable */
import { BinaryReader, BinaryWriter } from "@bufbuild/protobuf/wire";

export const protobufPackage = "lumera.claim";

/** Params defines the parameters for the module. */
export interface Params {
  enableClaims: boolean;
  claimEndTime: number;
  maxClaimsPerBlock: number;
}

function createBaseParams(): Params {
  return { enableClaims: false, claimEndTime: 0, maxClaimsPerBlock: 0 };
}

export const Params: MessageFns<Params> = {
  encode(message: Params, writer: BinaryWriter = new BinaryWriter()): BinaryWriter {
    if (message.enableClaims !== false) {
      writer.uint32(8).bool(message.enableClaims);
    }
    if (message.claimEndTime !== 0) {
      writer.uint32(24).int64(message.claimEndTime);
    }
    if (message.maxClaimsPerBlock !== 0) {
      writer.uint32(32).uint64(message.maxClaimsPerBlock);
    }
    return writer;
  },

  decode(input: BinaryReader | Uint8Array, length?: number): Params {
    const reader = input instanceof BinaryReader ? input : new BinaryReader(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseParams();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1: {
          if (tag !== 8) {
            break;
          }

          message.enableClaims = reader.bool();
          continue;
        }
        case 3: {
          if (tag !== 24) {
            break;
          }

          message.claimEndTime = longToNumber(reader.int64());
          continue;
        }
        case 4: {
          if (tag !== 32) {
            break;
          }

          message.maxClaimsPerBlock = longToNumber(reader.uint64());
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

  fromJSON(object: any): Params {
    return {
      enableClaims: isSet(object.enableClaims) ? globalThis.Boolean(object.enableClaims) : false,
      claimEndTime: isSet(object.claimEndTime) ? globalThis.Number(object.claimEndTime) : 0,
      maxClaimsPerBlock: isSet(object.maxClaimsPerBlock) ? globalThis.Number(object.maxClaimsPerBlock) : 0,
    };
  },

  toJSON(message: Params): unknown {
    const obj: any = {};
    if (message.enableClaims !== false) {
      obj.enableClaims = message.enableClaims;
    }
    if (message.claimEndTime !== 0) {
      obj.claimEndTime = Math.round(message.claimEndTime);
    }
    if (message.maxClaimsPerBlock !== 0) {
      obj.maxClaimsPerBlock = Math.round(message.maxClaimsPerBlock);
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<Params>, I>>(base?: I): Params {
    return Params.fromPartial(base ?? ({} as any));
  },
  fromPartial<I extends Exact<DeepPartial<Params>, I>>(object: I): Params {
    const message = createBaseParams();
    message.enableClaims = object.enableClaims ?? false;
    message.claimEndTime = object.claimEndTime ?? 0;
    message.maxClaimsPerBlock = object.maxClaimsPerBlock ?? 0;
    return message;
  },
};

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
