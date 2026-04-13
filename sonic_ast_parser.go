package astjson

import (
	"fmt"

	"github.com/bytedance/sonic/ast"
)

// ParseWithSonicAST parses JSON using sonic's native AST (ast.Parser) and
// converts the resulting ast.Node tree into an astjson Value tree.
//
// Compared to ParseWithSonic, this path skips the interface{} /
// map[string]interface{} / []interface{} intermediate representation. It
// walks sonic's own AST directly, so there is no boxing of primitives and
// no map construction — each JSON value is materialized only once (as a
// sonic Node) before being turned into the astjson Value.
func ParseWithSonicAST(data []byte) (*Value, error) {
	p := ast.NewParser(string(data))
	root, perr := p.Parse()
	if perr != 0 {
		return nil, fmt.Errorf("sonic ast parse: %s", perr.Error())
	}
	// Force full parse of lazy subtrees so downstream iteration sees
	// materialized children. Without this, ForEach still works but each
	// child is parsed on demand — LoadAll makes the cost comparable to
	// the other variants which eagerly produce the whole tree.
	if err := root.LoadAll(); err != nil {
		return nil, err
	}
	return sonicNodeToValue(&root)
}

func sonicNodeToValue(n *ast.Node) (*Value, error) {
	switch n.TypeSafe() {
	case ast.V_NULL, ast.V_NONE:
		return valueNull, nil
	case ast.V_TRUE:
		return valueTrue, nil
	case ast.V_FALSE:
		return valueFalse, nil
	case ast.V_STRING:
		s, err := n.StrictString()
		if err != nil {
			return nil, err
		}
		return StringValue(nil, s), nil
	case ast.V_NUMBER:
		num, err := n.Number()
		if err != nil {
			return nil, err
		}
		return NumberValue(nil, string(num)), nil
	case ast.V_ARRAY:
		arr := ArrayValue(nil)
		var iterErr error
		if err := n.ForEach(func(path ast.Sequence, node *ast.Node) bool {
			v, convErr := sonicNodeToValue(node)
			if convErr != nil {
				iterErr = convErr
				return false
			}
			AppendToArray(nil, arr, v)
			return true
		}); err != nil {
			return nil, err
		}
		return arr, iterErr
	case ast.V_OBJECT:
		obj := ObjectValue(nil)
		var iterErr error
		if err := n.ForEach(func(path ast.Sequence, node *ast.Node) bool {
			v, convErr := sonicNodeToValue(node)
			if convErr != nil {
				iterErr = convErr
				return false
			}
			obj.Set(nil, *path.Key, v)
			return true
		}); err != nil {
			return nil, err
		}
		return obj, iterErr
	default:
		return valueNull, nil
	}
}
