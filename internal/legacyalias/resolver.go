package legacyalias

import (
	"strings"

	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
)

// Resolver wraps a base protobuf resolver and transparently remaps legacy
// (pre-versioned) type names to their canonical counterparts before delegating
// to the base resolver.
type Resolver struct {
	base interface {
		protoregistry.MessageTypeResolver
		protoregistry.ExtensionTypeResolver
	}
	files   protodesc.Resolver
	aliases map[protoreflect.FullName]protoreflect.FullName
}

// WrapResolver returns the base resolver wrapped with legacy alias awareness.
// If no aliases are registered it simply returns the base resolver unchanged.
func WrapResolver(
	base interface {
		protoregistry.MessageTypeResolver
		protoregistry.ExtensionTypeResolver
	},
	files protodesc.Resolver,
) interface {
	protoregistry.MessageTypeResolver
	protoregistry.ExtensionTypeResolver
} {
	aliasSnapshot := Snapshot()
	if len(aliasSnapshot) == 0 {
		return base
	}
	return &Resolver{
		base:    base,
		files:   files,
		aliases: aliasSnapshot,
	}
}

func (r *Resolver) resolve(name protoreflect.FullName) protoreflect.FullName {
	if canonical, ok := r.aliases[name]; ok {
		return canonical
	}
	return name
}

func (r *Resolver) FindMessageByName(name protoreflect.FullName) (protoreflect.MessageType, error) {
	target := r.resolve(name)
	mt, err := r.base.FindMessageByName(target)
	if err == nil || err != protoregistry.NotFound {
		return mt, err
	}
	if r.files == nil {
		return nil, err
	}
	desc, derr := r.files.FindDescriptorByName(target)
	if derr != nil {
		return nil, err
	}
	md, ok := desc.(protoreflect.MessageDescriptor)
	if !ok {
		return nil, err
	}
	return dynamicpb.NewMessageType(md), nil
}

func (r *Resolver) FindMessageByURL(url string) (protoreflect.MessageType, error) {
	trimmed := strings.TrimPrefix(url, "/")
	if canonical := r.resolve(protoreflect.FullName(trimmed)); canonical != protoreflect.FullName(trimmed) {
		return r.FindMessageByName(canonical)
	}
	return r.base.FindMessageByURL(url)
}

func (r *Resolver) FindExtensionByName(name protoreflect.FullName) (protoreflect.ExtensionType, error) {
	return r.base.FindExtensionByName(name)
}

func (r *Resolver) FindExtensionByNumber(message protoreflect.FullName, field protoreflect.FieldNumber) (protoreflect.ExtensionType, error) {
	return r.base.FindExtensionByNumber(message, field)
}
