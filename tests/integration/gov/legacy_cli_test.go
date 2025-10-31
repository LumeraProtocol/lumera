package gov_test

import (
	"testing"

	gogoproto "github.com/cosmos/gogoproto/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/anypb"

	"cosmossdk.io/x/tx/signing"
	aminojson "cosmossdk.io/x/tx/signing/aminojson"

	"github.com/LumeraProtocol/lumera/internal/legacyalias"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"

	_ "github.com/LumeraProtocol/lumera/app" // ensure init hooks run
)

func TestAutoCLIEncoderHandlesLegacyProposalMessages(t *testing.T) {
	t.Helper()

	msg := &actiontypes.MsgUpdateParams{
		Authority: "authority",
	}
	msgBz, err := gogoproto.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal action message: %v", err)
	}

	files, err := gogoproto.MergedRegistry()
	if err != nil {
		t.Fatalf("MergedRegistry: %v", err)
	}

	wrapped := legacyalias.WrapResolver(protoregistry.GlobalTypes, files)
	typeResolver, ok := wrapped.(signing.TypeResolver)
	if !ok {
		t.Fatalf("wrapped resolver does not implement signing.TypeResolver")
	}
	if _, err := typeResolver.FindMessageByURL("/lumera.action.MsgUpdateParams"); err != nil {
		t.Fatalf("resolver failed to resolve legacy URL: %v", err)
	}

	queryDesc, err := files.FindDescriptorByName("cosmos.gov.v1.Query")
	if err != nil {
		t.Fatalf("find query descriptor: %v", err)
	}

	service := queryDesc.(protoreflect.ServiceDescriptor)
	method := service.Methods().ByName("Proposals")
	if method == nil {
		t.Fatalf("Proposals method not found")
	}

	responseType := dynamicpb.NewMessageType(method.Output())
	response := responseType.New()

	proposalsField := response.Descriptor().Fields().ByName("proposals")
	if proposalsField == nil {
		t.Fatalf("proposals field not found")
	}

	proposalType := dynamicpb.NewMessageType(proposalsField.Message())
	proposal := proposalType.New()

	messagesField := proposal.Descriptor().Fields().ByName("messages")
	if messagesField == nil {
		t.Fatalf("messages field not found")
	}

	legacyURL := "/lumera.action.MsgUpdateParams"
	anyMsg := &anypb.Any{TypeUrl: legacyURL, Value: msgBz}
	msgList := proposal.Mutable(messagesField).List()
	msgList.Append(protoreflect.ValueOfMessage(anyMsg.ProtoReflect()))

	proposalList := response.Mutable(proposalsField).List()
	proposalList.Append(protoreflect.ValueOfMessage(proposal))

	encoder := aminojson.NewEncoder(aminojson.EncoderOptions{
		FileResolver: files,
		TypeResolver: typeResolver,
		EnumAsString: true,
	})

	if _, err := encoder.Marshal(response.Interface()); err != nil {
		t.Fatalf("encoder marshal legacy proposal: %v", err)
	}
}
