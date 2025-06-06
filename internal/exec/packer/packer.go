package packer

import (
	"fmt"
	"math"
	"reflect"
	"strings"

	"github.com/graph-gophers/graphql-go/ast"
	"github.com/graph-gophers/graphql-go/decode"
	"github.com/graph-gophers/graphql-go/errors"
)

type packer interface {
	Pack(value interface{}) (reflect.Value, error)
}

type Builder struct {
	packerMap     map[typePair]*packerMapEntry
	structPackers []*StructPacker
}

type typePair struct {
	graphQLType  ast.Type
	resolverType reflect.Type
}

type packerMapEntry struct {
	packer  packer
	targets []*packer
}

func NewBuilder() *Builder {
	return &Builder{
		packerMap: make(map[typePair]*packerMapEntry),
	}
}

func (b *Builder) Finish() error {
	for _, entry := range b.packerMap {
		for _, target := range entry.targets {
			*target = entry.packer
		}
	}

	for _, p := range b.structPackers {
		p.defaultStruct = reflect.New(p.structType).Elem()
		for _, f := range p.fields {
			if defaultVal := f.def; defaultVal != nil {
				v, err := f.packer.Pack(defaultVal.Deserialize(nil))
				if err != nil {
					return err
				}
				p.defaultStruct.FieldByIndex(f.index).Set(v)
			}
		}
	}

	return nil
}

func (b *Builder) assignPacker(target *packer, schemaType ast.Type, reflectType reflect.Type) error {
	k := typePair{schemaType, reflectType}
	ref, ok := b.packerMap[k]
	if !ok {
		ref = &packerMapEntry{}
		b.packerMap[k] = ref
		var err error
		ref.packer, err = b.makePacker(schemaType, reflectType)
		if err != nil {
			return err
		}
	}
	ref.targets = append(ref.targets, target)
	return nil
}

func (b *Builder) makePacker(schemaType ast.Type, reflectType reflect.Type) (packer, error) {
	t, nonNull := unwrapNonNull(schemaType)
	if !nonNull {
		if reflectType.Kind() == reflect.Ptr {
			elemType := reflectType.Elem()
			addPtr := true
			if _, ok := t.(*ast.InputObject); ok {
				elemType = reflectType // keep pointer for input objects
				addPtr = false
			}
			elem, err := b.makeNonNullPacker(t, elemType)
			if err != nil {
				return nil, err
			}
			return &nullPacker{
				elemPacker: elem,
				valueType:  reflectType,
				addPtr:     addPtr,
			}, nil
		} else if isNullable(reflectType) {
			elemType := reflectType
			addPtr := false
			elem, err := b.makeNonNullPacker(t, elemType)
			if err != nil {
				return nil, err
			}
			return &nullPacker{
				elemPacker: elem,
				valueType:  reflectType,
				addPtr:     addPtr,
			}, nil
		} else {
			return nil, fmt.Errorf("%s is not a pointer or a nullable type", reflectType)
		}
	}

	return b.makeNonNullPacker(t, reflectType)
}

func (b *Builder) makeNonNullPacker(schemaType ast.Type, reflectType reflect.Type) (packer, error) {
	if u, ok := reflect.New(reflectType).Interface().(decode.Unmarshaler); ok {
		if !u.ImplementsGraphQLType(schemaType.String()) {
			return nil, fmt.Errorf("can not unmarshal %s into %s", schemaType, reflectType)
		}
		return &unmarshalerPacker{
			ValueType: reflectType,
		}, nil
	}

	switch t := schemaType.(type) {
	case *ast.ScalarTypeDefinition:
		return &ValuePacker{
			ValueType: reflectType,
		}, nil

	case *ast.EnumTypeDefinition:
		if reflectType.Kind() != reflect.String {
			return nil, fmt.Errorf("wrong type, expected %s", reflect.String)
		}
		return &ValuePacker{
			ValueType: reflectType,
		}, nil

	case *ast.InputObject:
		e, err := b.MakeStructPacker(t.Values, reflectType)
		if err != nil {
			return nil, err
		}
		return e, nil

	case *ast.List:
		if reflectType.Kind() != reflect.Slice {
			return nil, fmt.Errorf("expected slice, got %s", reflectType)
		}
		p := &listPacker{
			sliceType: reflectType,
		}
		if err := b.assignPacker(&p.elem, t.OfType, reflectType.Elem()); err != nil {
			return nil, err
		}
		return p, nil

	case *ast.ObjectTypeDefinition, *ast.InterfaceTypeDefinition, *ast.Union:
		return nil, fmt.Errorf("type of kind %s can not be used as input", t.Kind())

	default:
		panic("unreachable")
	}
}

func (b *Builder) MakeStructPacker(values []*ast.InputValueDefinition, typ reflect.Type) (*StructPacker, error) {
	structType := typ
	usePtr := false
	if typ.Kind() == reflect.Ptr {
		structType = typ.Elem()
		usePtr = true
	}
	if structType.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected struct or pointer to struct, got %s (hint: missing `args struct { ... }` wrapper for field arguments?)", typ)
	}

	var fields []*structPackerField
	for _, v := range values {
		name := v.Name.Name
		fe := &structPackerField{name: name, def: v.Default}
		fx := func(n string) bool {
			return strings.EqualFold(stripUnderscore(n), stripUnderscore(name))
		}

		sf, ok := structType.FieldByNameFunc(fx)
		if !ok {
			return nil, fmt.Errorf("%s does not define field %q (hint: missing `args struct { ... }` wrapper for field arguments, or missing field on input struct)", typ, name)
		}
		if sf.PkgPath != "" {
			return nil, fmt.Errorf("field %q must be exported", sf.Name)
		}
		if _, ok := v.Type.(*ast.NonNull); ok {
			if sf.Type.Kind() == reflect.Ptr {
				return nil, fmt.Errorf("field %q must be a non-pointer since the parameter is required", sf.Name)
			}
		}

		fe.index = sf.Index

		ft := v.Type
		if v.Default != nil {
			ft, _ = unwrapNonNull(ft)
			ft = &ast.NonNull{OfType: ft}
		}

		if err := b.assignPacker(&fe.packer, ft, sf.Type); err != nil {
			return nil, fmt.Errorf("field %q: %s", sf.Name, err)
		}

		fields = append(fields, fe)
	}

	p := &StructPacker{
		structType: structType,
		usePtr:     usePtr,
		fields:     fields,
	}
	b.structPackers = append(b.structPackers, p)
	return p, nil
}

type StructPacker struct {
	structType    reflect.Type
	usePtr        bool
	defaultStruct reflect.Value
	fields        []*structPackerField
}

type structPackerField struct {
	name   string
	index  []int
	def    ast.Value
	packer packer
}

func (p *StructPacker) Pack(value interface{}) (reflect.Value, error) {
	if value == nil {
		return reflect.Value{}, errors.Errorf("got null for non-null")
	}

	values := value.(map[string]interface{})
	v := reflect.New(p.structType)
	v.Elem().Set(p.defaultStruct)
	for _, f := range p.fields {
		if value, ok := values[f.name]; ok {
			packed, err := f.packer.Pack(value)
			if err != nil {
				return reflect.Value{}, err
			}
			v.Elem().FieldByIndex(f.index).Set(packed)
		}
	}
	if !p.usePtr {
		return v.Elem(), nil
	}
	return v, nil
}

type listPacker struct {
	sliceType reflect.Type
	elem      packer
}

func (e *listPacker) Pack(value interface{}) (reflect.Value, error) {
	list, ok := value.([]interface{})
	if !ok {
		list = []interface{}{value}
	}

	v := reflect.MakeSlice(e.sliceType, len(list), len(list))
	for i := range list {
		packed, err := e.elem.Pack(list[i])
		if err != nil {
			return reflect.Value{}, err
		}
		v.Index(i).Set(packed)
	}
	return v, nil
}

type nullPacker struct {
	elemPacker packer
	valueType  reflect.Type
	addPtr     bool
}

func (p *nullPacker) Pack(value interface{}) (reflect.Value, error) {
	if value == nil && !isNullable(p.valueType) {
		return reflect.Zero(p.valueType), nil
	}

	v, err := p.elemPacker.Pack(value)
	if err != nil {
		return reflect.Value{}, err
	}

	if p.addPtr {
		ptr := reflect.New(p.valueType.Elem())
		ptr.Elem().Set(v)
		return ptr, nil
	}

	return v, nil
}

type ValuePacker struct {
	ValueType reflect.Type
}

func (p *ValuePacker) Pack(value interface{}) (reflect.Value, error) {
	if value == nil {
		return reflect.Value{}, errors.Errorf("got null for non-null")
	}

	coerced, err := unmarshalInput(p.ValueType, value)
	if err != nil {
		return reflect.Value{}, fmt.Errorf("could not unmarshal %#v (%T) into %s: %s", value, value, p.ValueType, err)
	}
	return reflect.ValueOf(coerced), nil
}

type unmarshalerPacker struct {
	ValueType reflect.Type
}

func (p *unmarshalerPacker) Pack(value interface{}) (reflect.Value, error) {
	if value == nil && !isNullable(p.ValueType) {
		return reflect.Value{}, errors.Errorf("got null for non-null")
	}

	v := reflect.New(p.ValueType)
	if err := v.Interface().(decode.Unmarshaler).UnmarshalGraphQL(value); err != nil {
		return reflect.Value{}, err
	}
	return v.Elem(), nil
}

func unmarshalInput(typ reflect.Type, input interface{}) (interface{}, error) {
	if reflect.TypeOf(input) == typ {
		return input, nil
	}

	switch typ.Kind() {
	case reflect.Int32:
		switch input := input.(type) {
		case int:
			if input < math.MinInt32 || input > math.MaxInt32 {
				return nil, fmt.Errorf("not a 32-bit integer")
			}
			return int32(input), nil
		case float64:
			coerced := int32(input)
			if input < math.MinInt32 || input > math.MaxInt32 || float64(coerced) != input {
				return nil, fmt.Errorf("not a 32-bit integer")
			}
			return coerced, nil
		}

	case reflect.Float64:
		switch input := input.(type) {
		case int32:
			return float64(input), nil
		case int:
			return float64(input), nil
		}

	case reflect.String:
		if reflect.TypeOf(input).ConvertibleTo(typ) {
			return reflect.ValueOf(input).Convert(typ).Interface(), nil
		}
	}

	return nil, fmt.Errorf("incompatible type: %s", reflect.TypeOf(input))
}

func unwrapNonNull(t ast.Type) (ast.Type, bool) {
	if nn, ok := t.(*ast.NonNull); ok {
		return nn.OfType, true
	}
	return t, false
}

func stripUnderscore(s string) string {
	return strings.ReplaceAll(s, "_", "")
}

// NullUnmarshaller is an unmarshaller that can handle a nil input
type NullUnmarshaller interface {
	decode.Unmarshaler
	Nullable()
}

func isNullable(t reflect.Type) bool {
	_, ok := reflect.New(t).Interface().(NullUnmarshaller)
	return ok
}
