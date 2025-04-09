// Code generated by protoc-gen-ts_proto. DO NOT EDIT.
// versions:
//   protoc-gen-ts_proto  v2.6.1
//   protoc               unknown
// source: lumera/supernode/super_node.proto

/* eslint-disable */
import { BinaryReader, BinaryWriter } from "@bufbuild/protobuf/wire";
import { Evidence } from "./evidence";
import { IPAddressHistory } from "./ip_address_history";
import { MetricsAggregate } from "./metrics_aggregate";
import { SuperNodeStateRecord } from "./supernode_state";

export const protobufPackage = "lumera.supernode";

export interface SuperNode {
  validatorAddress: string;
  states: SuperNodeStateRecord[];
  evidence: Evidence[];
  prevIpAddresses: IPAddressHistory[];
  version: string;
  metrics: MetricsAggregate | undefined;
  supernodeAccount: string;
}

function createBaseSuperNode(): SuperNode {
  return {
    validatorAddress: "",
    states: [],
    evidence: [],
    prevIpAddresses: [],
    version: "",
    metrics: undefined,
    supernodeAccount: "",
  };
}

export const SuperNode: MessageFns<SuperNode> = {
  encode(message: SuperNode, writer: BinaryWriter = new BinaryWriter()): BinaryWriter {
    if (message.validatorAddress !== "") {
      writer.uint32(10).string(message.validatorAddress);
    }
    for (const v of message.states) {
      SuperNodeStateRecord.encode(v!, writer.uint32(18).fork()).join();
    }
    for (const v of message.evidence) {
      Evidence.encode(v!, writer.uint32(26).fork()).join();
    }
    for (const v of message.prevIpAddresses) {
      IPAddressHistory.encode(v!, writer.uint32(34).fork()).join();
    }
    if (message.version !== "") {
      writer.uint32(42).string(message.version);
    }
    if (message.metrics !== undefined) {
      MetricsAggregate.encode(message.metrics, writer.uint32(50).fork()).join();
    }
    if (message.supernodeAccount !== "") {
      writer.uint32(58).string(message.supernodeAccount);
    }
    return writer;
  },

  decode(input: BinaryReader | Uint8Array, length?: number): SuperNode {
    const reader = input instanceof BinaryReader ? input : new BinaryReader(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseSuperNode();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1: {
          if (tag !== 10) {
            break;
          }

          message.validatorAddress = reader.string();
          continue;
        }
        case 2: {
          if (tag !== 18) {
            break;
          }

          message.states.push(SuperNodeStateRecord.decode(reader, reader.uint32()));
          continue;
        }
        case 3: {
          if (tag !== 26) {
            break;
          }

          message.evidence.push(Evidence.decode(reader, reader.uint32()));
          continue;
        }
        case 4: {
          if (tag !== 34) {
            break;
          }

          message.prevIpAddresses.push(IPAddressHistory.decode(reader, reader.uint32()));
          continue;
        }
        case 5: {
          if (tag !== 42) {
            break;
          }

          message.version = reader.string();
          continue;
        }
        case 6: {
          if (tag !== 50) {
            break;
          }

          message.metrics = MetricsAggregate.decode(reader, reader.uint32());
          continue;
        }
        case 7: {
          if (tag !== 58) {
            break;
          }

          message.supernodeAccount = reader.string();
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

  fromJSON(object: any): SuperNode {
    return {
      validatorAddress: isSet(object.validatorAddress) ? globalThis.String(object.validatorAddress) : "",
      states: globalThis.Array.isArray(object?.states)
        ? object.states.map((e: any) => SuperNodeStateRecord.fromJSON(e))
        : [],
      evidence: globalThis.Array.isArray(object?.evidence) ? object.evidence.map((e: any) => Evidence.fromJSON(e)) : [],
      prevIpAddresses: globalThis.Array.isArray(object?.prevIpAddresses)
        ? object.prevIpAddresses.map((e: any) => IPAddressHistory.fromJSON(e))
        : [],
      version: isSet(object.version) ? globalThis.String(object.version) : "",
      metrics: isSet(object.metrics) ? MetricsAggregate.fromJSON(object.metrics) : undefined,
      supernodeAccount: isSet(object.supernodeAccount) ? globalThis.String(object.supernodeAccount) : "",
    };
  },

  toJSON(message: SuperNode): unknown {
    const obj: any = {};
    if (message.validatorAddress !== "") {
      obj.validatorAddress = message.validatorAddress;
    }
    if (message.states?.length) {
      obj.states = message.states.map((e) => SuperNodeStateRecord.toJSON(e));
    }
    if (message.evidence?.length) {
      obj.evidence = message.evidence.map((e) => Evidence.toJSON(e));
    }
    if (message.prevIpAddresses?.length) {
      obj.prevIpAddresses = message.prevIpAddresses.map((e) => IPAddressHistory.toJSON(e));
    }
    if (message.version !== "") {
      obj.version = message.version;
    }
    if (message.metrics !== undefined) {
      obj.metrics = MetricsAggregate.toJSON(message.metrics);
    }
    if (message.supernodeAccount !== "") {
      obj.supernodeAccount = message.supernodeAccount;
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<SuperNode>, I>>(base?: I): SuperNode {
    return SuperNode.fromPartial(base ?? ({} as any));
  },
  fromPartial<I extends Exact<DeepPartial<SuperNode>, I>>(object: I): SuperNode {
    const message = createBaseSuperNode();
    message.validatorAddress = object.validatorAddress ?? "";
    message.states = object.states?.map((e) => SuperNodeStateRecord.fromPartial(e)) || [];
    message.evidence = object.evidence?.map((e) => Evidence.fromPartial(e)) || [];
    message.prevIpAddresses = object.prevIpAddresses?.map((e) => IPAddressHistory.fromPartial(e)) || [];
    message.version = object.version ?? "";
    message.metrics = (object.metrics !== undefined && object.metrics !== null)
      ? MetricsAggregate.fromPartial(object.metrics)
      : undefined;
    message.supernodeAccount = object.supernodeAccount ?? "";
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
