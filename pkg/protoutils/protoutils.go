package protoutils

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// ApplyFieldMaskToNewMessage creates a new message containing only the fields specified in the FieldMask.
// If mask is nil then source is returned in its original unalterned state
func ApplyFieldMaskToNewMessage(source proto.Message, mask *fieldmaskpb.FieldMask) (proto.Message, error) {
	if mask == nil {
		return source, nil
	}

	newMessage := proto.Clone(source)
	newMessageProto := newMessage.ProtoReflect()
	sourceProto := source.ProtoReflect()

	// Clear all fields initially
	newMessageProto.Range(func(field protoreflect.FieldDescriptor, _ protoreflect.Value) bool {
		newMessageProto.Clear(field)
		return true
	})

	// Apply each field specified in the FieldMask
	for _, path := range mask.Paths {
		err := ApplyNestedField(newMessageProto, sourceProto, strings.Split(path, "."))
		if err != nil {
			return nil, err
		}
	}

	return newMessage, nil
}

// FieldMaskTriggersUpdate checks if the given mask is considered to be a config change and if it should trigger an update event
func FieldMaskTriggersUpdate(mask *fieldmaskpb.FieldMask) bool {
	for _, p := range mask.GetPaths() {
		if strings.Contains(p, "config") {
			return true
		}
	}
	return false
}

// ApplyNestedField sets the value of a nested field in the target message based on the source message.
func ApplyNestedField(target, source protoreflect.Message, path []string) error {
	if len(path) == 0 {
		return nil
	}

	// Look up the field descriptor for the current level in the path
	field := source.Descriptor().Fields().ByName(protoreflect.Name(path[0]))
	if field == nil {
		return fmt.Errorf("field %q not found in target message", path[0])
	}

	// If we are at the final field in the path, set it directly
	if len(path) == 1 {
		if source.Has(field) {
			target.Set(field, source.Get(field))
		}
		return nil
	}

	// Recurse into the nested message
	if field.Message() == nil {
		return fmt.Errorf("field %q is not a message type", path[0])
	}

	// Ensure the target has an initialized message at this field
	if !target.Has(field) {
		target.Set(field, target.NewField(field))
	}

	return ApplyNestedField(target.Mutable(field).Message(), source.Get(field).Message(), path[1:])
}
