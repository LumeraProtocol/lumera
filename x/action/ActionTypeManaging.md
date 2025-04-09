This document describes how the Lumera Action module handles multiple action types and offers a step-by-step guide for future developers to add new types. Each action type uses its own specialized metadata and maintains validation logic that is kept separate from the core Action management code.

---
## 1. Introduction

The Lumera Action module supports multiple action types (such as Cascade and Sense), each with its own metadata structure and validation rules. The system can be extended with new action types by registering a handler for the desired type, ensuring minimal changes to the core module.

---
## 2. Architecture Overview

### 2.1 Action Structure

The primary protobuf definition for an Action includes a field for type-specific metadata, stored as bytes:

```protobuf
message Action {
  string creator = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  string actionID = 2;
  ActionType actionType = 3;
  bytes metadata = 4; // Type-specific metadata
  string price = 5 [(gogoproto.customtype) = "github.com/cosmos/cosmos-sdk/types.Coin"];  
  int64 expirationTime = 6;  
  ActionState state = 7;  
  int64 blockHeight = 8;  
  repeated string superNodes = 9 [(cosmos_proto.scalar) = "cosmos.ValidatorAddressString"];  
}
```

Users of the Action module can decode and handle these bytes according to each action type’s metadata structure.

### 2.2 Action Type Registry and Validation

A registry-based approach determines which handler or validator applies to each action type. This involves two main parts:
1. **An Enum Representing the ActionType**  
   In protobuf or a similar definition, each supported action type (e.g., CASCADE, SENSE) is identified by a constant or enum value.
2. **Action Type Registration**  
   During initialization, each action type is registered with a validator that implements specialized logic.

Action specific code for Type Validation must be implemented in the `types` package.
That code is intended to be used in the `ValidateBasic()` method of the action specific `Message`. It must be light weight and it doesn't have an access to the context.

```go
// x/action/types/action_type.go
package types

import (...)

type ActionTypeValidator interface {
    // ActionType corresponds to a particular ActionType enum value.
    ActionType() actionapi.ActionType

    // ValidateBasic performs validation checks specific to this action type.
    ValidateBasic(metadataStr string, msgType common.MessageType) error
}

var validatorRegistry = make(map[string]ActionTypeValidator)

func RegisterValidator(v ActionTypeValidator, aliases ...string) {
    for _, alias := range aliases {
        key := strings.ToUpper(alias)
        validatorRegistry[key] = v
    }
}

// Example usage in your init function:
// RegisterValidator(&CascadeValidator{}, "CASCADE", "ACTION_TYPE_CASCADE")

func DoActionValidation(metadata string, actionTypeStr string, msgType common.MessageType) error {
    // ...
    // Use validatorRegistry to find the right validator by actionTypeStr.
    // ...
    return nil
}
```

When an action is processed, the system looks up the validator for its action type, then calls the relevant validation methods.

### 2.3 Metadata Handlers

For more advanced fields or parsing, each action type can also have a “handler” interface. Handlers often provide methods for:

- Validating metadata.
- Converting between JSON and binary (protobuf-friendly) formats.
- Processing any transformations needed for that action type.

Action specific code for Metadata Hadnler must be implemented in the `keeper` package.
This is intended to be called from the keeper's methods and it does have access to the context and keeper.

```go
// x/action/keeper/metadata_handler.go
package common

import (...)

type MessageType int

const (
    MsgRequestAction  MessageType = iota
    MsgFinalizeAction
    // ...
)

type MetadataHandler interface {
    Validate(data []byte, msgType MessageType) error
    Process(data []byte, msgType MessageType) ([]byte, error)
    GetProtoMessageType() reflect.Type
    ConvertJSONToProtobuf(jsonData []byte) ([]byte, error)
    ConvertProtobufToJSON(protobufData []byte) ([]byte, error)
}
```

Developers can define specialized handler implementations for each action type and register them in a lookup mechanism (e.g., a `MetadataRegistry`), so that when an action is processed, the system invokes the correct handler based on the action’s type.

### 2.4 Example: Cascade Action Validation

Below is an examples of a Cascade type validator and metadata handler:
#### 2.4.1 Type validation

```go
// x/action/types/action_type_cascade.go
package types

import (...)

type CascadeValidator struct{}

func init() {
    RegisterValidator(&CascadeValidator{},
        "CASCADE",
        "ACTION_TYPE_CASCADE",
    )
}

func (v *CascadeValidator) ActionType() actionapi.ActionType {
    return actionapi.ActionType_ACTION_TYPE_CASCADE
}

func (v *CascadeValidator) ValidateBasic(metadataStr string, msgType common.MessageType) error {
    var metadata actionapi.CascadeMetadata
    if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
        return fmt.Errorf("failed to unmarshal cascade metadata: %w", err)
    }

    if msgType == common.MsgRequestAction {
        if metadata.DataHash == "" {
            return fmt.Errorf("data_hash is required for cascade metadata")
        }
        // Additional checks...
    }
    if msgType == common.MsgFinalizeAction {
        if len(metadata.RqIdsIds) == 0 {
            return fmt.Errorf("rq_ids_ids is required for cascade metadata")
        }
        // Additional checks...
    }
    return nil
}
```

When an Action of type CASCADE is submitted, the system retrieves the corresponding validator from the registry. It unmarshals the JSON metadata and performs specific checks. If any required field is missing, the validator returns an error.

#### 2.4.2 Metdata Handler

```go
// x/action/keeper/action_handlers.go
package keeper  
  
import (...)  
  
// InitializeActionRegistry sets up the action registry with default handlers
func (k *Keeper) InitializeActionRegistry() *ActionRegistry {  
    registry := NewActionRegistry(k)  
  
    // Register handlers for existing action types  
    registry.RegisterHandler(actionapi.ActionType_ACTION_TYPE_SENSE, NewSenseActionHandler(k))  
    registry.RegisterHandler(actionapi.ActionType_ACTION_TYPE_CASCADE, NewCascadeActionHandler(k))  
  
    return registry  
}
```

```go
// x/action/keeper/action_cascade.go
package keeper  
  
import (...)  
  
type CascadeActionHandler struct {  
    keeper *Keeper // Reference to the keeper for logger and other services}  
  
// NewCascadeActionHandler creates a new CascadeActionHandler  
func NewCascadeActionHandler(k *Keeper) *CascadeActionHandler {  
    return &CascadeActionHandler{  
       keeper: k,  
    }  
}  
  
// Validate validates CascadeMetadata based on message type
func (h CascadeActionHandler) Validate(data []byte, msgType common.MessageType) error {
...
}

```

---

## 3. Step-by-Step: Adding a New Action Type

To add a new action type (for example, "MyCustomAction"), follow these steps:

#### 1. Protobuf Definition

In your `.proto` file (or equivalent), define a new enum value (e.g., `ACTION_TYPE_MY_ACTION`) and a corresponding metadata message (e.g., `MyActionMetadata`).

   ```protobuf
   enum ActionType {
       ACTION_TYPE_UNSPECIFIED = 0;
       ACTION_TYPE_CASCADE = 1;
       ACTION_TYPE_SENSE = 2;
       ACTION_TYPE_MY_CUSTOM = 3;  // <-- newly added
   }

   message MyActionMetadata {
       string fieldA = 1;
       int32 fieldB = 2;
       // Additional fields...
   }
   ```
#### 2. Create a Type Validator

Create new file `x/action/types/action_type_my_action.go`

   ```go
   type MyActionValidator struct{}

   func init() {
       RegisterValidator(&MyCustomValidator{}, "MY_ACTION", "ACTION_TYPE_MY_ACTION")
   }

   func (v *MyActionValidator) ActionType() actionapi.ActionType {
       return actionapi.ActionType_ACTION_TYPE_MY_ACTION
   }

   func (v *MyActionValidator) ValidateBasic(metadataStr string, msgType common.MessageType) error {
       var metadata actionapi.MyActionMetadata
       if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
           return fmt.Errorf("failed to unmarshal my action metadata: %w", err)
       }
       // Perform any required validations...
       return nil
   }
   ```
* Ensure your validator or handler is registered in an `init()` function or a dedicated registration step. Confirm that your new action type’s enum value and aliases map to the correct validator.
* Actions typically go through one or more transaction messages (e.g., `MsgRequestAction`). Verify that any place in the code referencing the existing actions can handle your new type or otherwise route to your new validator.
* Create unit tests ensuring your new action type’s metadata is correctly validated. Test edge cases (missing fields, invalid data, etc.) to confirm that your validator or handler returns appropriate errors
#### 3. Create Metadata handler

Create new file `x/action/keeper/action_my_action.go`

```go
// x/action/keeper/action_cascade.go
package keeper  
  
import (...)  
  
type MyActionHandler struct {  
    keeper *Keeper // Reference to the keeper for logger and other services}  
  
// NewCascadeActionHandler creates a new MyActionHandler  
func NewMyActionHandler(k *Keeper) *MyActionHandler {  
    return &MyActionHandler{  
       keeper: k,  
    }  
}  
  
// Validate validates MyActionMetadata based on message type
func (h MyActionHandler) Validate(data []byte, msgType common.MessageType) error {
...
}

...
```

#### 4. Register new metadata handler

```go
// x/action/keeper/action_handlers.go
package keeper  
  
import (...)  
  
// InitializeActionRegistry sets up the action registry with default handlers
func (k *Keeper) InitializeActionRegistry() *ActionRegistry {  
    registry := NewActionRegistry(k)  
  
    // Register handlers for existing action types  
    registry.RegisterHandler(actionapi.ActionType_ACTION_TYPE_SENSE, NewSenseActionHandler(k))  
    registry.RegisterHandler(actionapi.ActionType_ACTION_TYPE_CASCADE, NewCascadeActionHandler(k))  
    registry.RegisterHandler(actionapi.ActionType_ACTION_TYPE_MY_ACTION, NewMyActionHandler(k))  // <--- NEW ACTION
  
    return registry  
}
```

---

## 4. Summary

The Lumera Action module uses a straightforward architecture where each action has a general set of fields (creator, action ID, etc.) plus a bytes field for its metadata. Action-specific validation and processing logic is kept in dedicated validator or handler components, which are registered under the action’s enum type. This modular design allows developers to introduce new action types without making significant changes to existing logic.

Use the examples in this document as a reference for creating new metadata structures, registering them in the action type registry, and integrating with the Lumera Action workflow.