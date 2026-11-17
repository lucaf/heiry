package heiry

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

type contractEmpty interface{}
type contractPublicA interface{ Read() int }
type contractPublicB interface {
	Read() int
	Write(int) error
}
type contractPrivateA interface{ Secret() string }
type contractPrivateB interface {
	Secret() string
	Transform(string) string
}
type contractRead interface{ Read() int }
type contractReadAlias = contractRead
type contractReadOther interface{ Read() int }
type contractWrite interface{ Write(int) error }
type contractSecret interface{ Secret() string }
type contractTransform interface{ Transform(string) string }
type contractReadVariadic interface{ Read(...int) int }
type contractPublicChanged interface{ Read(string) int }
type contractReadAndWrite interface {
	Read() int
	Write(int) error
}
type contractCollision interface{ Read() int }
type contractMovedPrivate interface{ Read() int }
type contractNarrow interface{ Write(int) error }
type contractBadMethod interface{ hidden() }
type contractAggregate interface {
	Read() int
	Write(int) error
}

type contractProvider struct{}

func (*contractProvider) Read() int                 { return 1 }
func (*contractProvider) Write(int) error           { return nil }
func (*contractProvider) Secret() string            { return "secret" }
func (*contractProvider) Transform(s string) string { return s }

func TestStage4ContractExtensionAndTargetCoverage(t *testing.T) {
	provider := reflect.TypeFor[*contractProvider]()
	levels := []contractLevel{
		{public: reflect.TypeFor[contractPublicA](), private: reflect.TypeFor[contractPrivateA](), bindings: []contractBinding{
			{capability: reflect.TypeFor[contractReadAlias](), provider: provider},
			{capability: reflect.TypeFor[contractSecret](), provider: provider},
		}},
		{public: reflect.TypeFor[contractPublicB](), private: reflect.TypeFor[contractPrivateB](), bindings: []contractBinding{
			{capability: reflect.TypeFor[contractWrite](), provider: provider},
		}},
		{bindings: []contractBinding{{capability: reflect.TypeFor[contractTransform](), provider: provider}}},
	}
	if _, err := validateContracts(levels, 0); err != nil {
		t.Fatalf("standalone A: %v", err)
	}
	if _, err := validateContracts(levels, 1); !errors.Is(err, &Error{Code: MissingMethods}) || !strings.Contains(err.Error(), "Transform") {
		t.Fatalf("abstract B = %v", err)
	}
	result, err := validateContracts(levels, 2)
	if err != nil || len(result.known) != 4 {
		t.Fatalf("complete C = (%v, %v)", result, err)
	}
}

func TestStage4ExplicitEmptyRootAndMissingMethodsOrder(t *testing.T) {
	levels := []contractLevel{{public: reflect.TypeFor[contractPublicB](), private: reflect.TypeFor[contractPrivateB]()}}
	_, err := validateContracts(levels, 0)
	if !errors.Is(err, &Error{Code: MissingMethods}) || err == nil {
		t.Fatalf("missing methods = %v", err)
	}
	last := -1
	for _, name := range []string{"Read", "Secret", "Transform", "Write"} {
		position := strings.Index(err.Error(), " "+name+" capability")
		if position <= last {
			t.Fatalf("missing %s from %v", name, err)
		}
		last = position
	}
	if _, err := validateContracts([]contractLevel{{public: reflect.TypeFor[contractEmpty](), private: reflect.TypeFor[contractEmpty]()}}, 0); err != nil {
		t.Fatalf("explicit empty root: %v", err)
	}
}

func TestStage4ContractAndCapabilityDiagnostics(t *testing.T) {
	provider := reflect.TypeFor[*contractProvider]()
	base := contractLevel{public: reflect.TypeFor[contractPublicA](), private: reflect.TypeFor[contractEmpty](), bindings: []contractBinding{{capability: reflect.TypeFor[contractRead](), provider: provider}}}
	tests := []struct {
		name   string
		levels []contractLevel
		code   Code
	}{
		{"missing root pair", []contractLevel{{public: reflect.TypeFor[contractPublicA]()}}, InvalidContract},
		{"narrowing", []contractLevel{base, {public: reflect.TypeFor[contractNarrow](), private: reflect.TypeFor[contractEmpty]()}}, ContractExtension},
		{"contract signature change", []contractLevel{base, {public: reflect.TypeFor[contractPublicChanged](), private: reflect.TypeFor[contractEmpty]()}}, ContractExtension},
		{"category move", []contractLevel{base, {public: reflect.TypeFor[contractEmpty](), private: reflect.TypeFor[contractMovedPrivate]()}}, ContractExtension},
		{"collision", []contractLevel{{public: reflect.TypeFor[contractPublicA](), private: reflect.TypeFor[contractCollision]()}}, ContractCollision},
		{"unexported", []contractLevel{{public: reflect.TypeFor[contractBadMethod](), private: reflect.TypeFor[contractEmpty]()}}, InvalidContract},
		{"undeclared", []contractLevel{{public: reflect.TypeFor[contractEmpty](), private: reflect.TypeFor[contractEmpty](), bindings: base.bindings}}, UndeclaredOperation},
		{"aggregate capability", []contractLevel{{public: reflect.TypeFor[contractReadAndWrite](), private: reflect.TypeFor[contractEmpty](), bindings: []contractBinding{{capability: reflect.TypeFor[contractAggregate](), provider: provider}}}}, InvalidCapability},
		{"noninterface capability", []contractLevel{{public: reflect.TypeFor[contractPublicA](), private: reflect.TypeFor[contractEmpty](), bindings: []contractBinding{{capability: reflect.TypeFor[int](), provider: provider}}}}, InvalidCapability},
		{"empty capability", []contractLevel{{public: reflect.TypeFor[contractPublicA](), private: reflect.TypeFor[contractEmpty](), bindings: []contractBinding{{capability: reflect.TypeFor[contractEmpty](), provider: provider}}}}, InvalidCapability},
		{"variadic mismatch", []contractLevel{{public: reflect.TypeFor[contractPublicA](), private: reflect.TypeFor[contractEmpty](), bindings: []contractBinding{{capability: reflect.TypeFor[contractReadVariadic](), provider: provider}}}}, SignatureMismatch},
		{"duplicate", []contractLevel{{public: reflect.TypeFor[contractPublicA](), private: reflect.TypeFor[contractEmpty](), bindings: append(append([]contractBinding{}, base.bindings...), base.bindings...)}}, DuplicateBinding},
		{"distinct identity", []contractLevel{base, {bindings: []contractBinding{{capability: reflect.TypeFor[contractReadOther](), provider: provider}}}}, CapabilityIdentity},
		{"wrong provider", []contractLevel{{public: reflect.TypeFor[contractPublicA](), private: reflect.TypeFor[contractEmpty](), bindings: []contractBinding{{capability: reflect.TypeFor[contractRead](), provider: reflect.TypeFor[*stage2B]()}}}}, SignatureMismatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateContracts(tt.levels, len(tt.levels)-1)
			if !errors.Is(err, &Error{Code: tt.code}) {
				t.Fatalf("error = %v, want %s", err, tt.code)
			}
		})
	}
}

func TestStage4StandaloneAncestorCannotUseDescendantBinding(t *testing.T) {
	levels := []contractLevel{
		{public: reflect.TypeFor[contractPublicA](), private: reflect.TypeFor[contractEmpty]()},
		{bindings: []contractBinding{{capability: reflect.TypeFor[contractRead](), provider: reflect.TypeFor[*contractProvider]()}}},
	}
	if _, err := validateContracts(levels, 0); !errors.Is(err, &Error{Code: MissingMethods}) {
		t.Fatalf("standalone A error = %v", err)
	}
	if _, err := validateContracts(levels, 1); err != nil {
		t.Fatalf("complete B error = %v", err)
	}
}

func TestStage4ExtensionDiagnosticOrder(t *testing.T) {
	levels := []contractLevel{
		{public: reflect.TypeFor[contractPublicB](), private: reflect.TypeFor[contractEmpty]()},
		{public: reflect.TypeFor[contractEmpty](), private: reflect.TypeFor[contractEmpty]()},
	}
	for range 50 {
		_, err := validateContracts(levels, 1)
		var typed *Error
		if !errors.As(err, &typed) || typed.Code != ContractExtension || typed.Name != "Read" {
			t.Fatalf("first extension diagnostic = %v, want Read", err)
		}
	}
}
