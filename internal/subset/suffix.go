package subset

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
)

// sliceSuffix recognizes the only slice syntax the borrow checker can plan:
// a standalone, one-argument call of a local function with no results.
func (c *checker) sliceSuffix(call *ast.CallExpr, fun ast.Expr, sig *types.Signature) bool {
	info := c.pkg.Info
	id, _ := fun.(*ast.Ident)
	fn, _ := info.Uses[id].(*types.Func)
	if fn == nil || fn.Pkg() != c.pkg.Types || sig.Results().Len() != 0 || len(call.Args) != 1 {
		return false
	}
	s, ok := ast.Unparen(call.Args[0]).(*ast.SliceExpr)
	if !ok || s.Low == nil || s.High != nil || s.Slice3 {
		return false
	}
	intType := types.Typ[types.Int]
	intSlice := types.NewSlice(intType)
	base, _ := ast.Unparen(s.X).(*ast.Ident)
	v, _ := info.Uses[base].(*types.Var)
	if v == nil || v.Parent() == v.Pkg().Scope() || !types.Identical(v.Type(), intSlice) || !types.Identical(sig.Params().At(0).Type(), intSlice) {
		return false
	}
	low := ast.Unparen(s.Low)
	switch x := low.(type) {
	case *ast.Ident:
		switch v := info.Uses[x].(type) {
		case *types.Var:
			return v.Parent() != v.Pkg().Scope() && types.Identical(v.Type(), intType)
		case *types.Const:
			return integerBound(v.Val())
		}
	case *ast.SelectorExpr:
		v, ok := info.Uses[x.Sel].(*types.Const)
		return ok && integerBound(v.Val())
	case *ast.BasicLit:
		return x.Kind == token.INT && integerBound(info.Types[x].Value)
	}
	return false
}

func integerBound(v constant.Value) bool {
	if v.Kind() != constant.Int {
		return false
	}
	_, ok := constant.Int64Val(v)
	return ok
}
