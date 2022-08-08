package printer

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"

	"github.com/jig/lisp/marshaler"
	"github.com/jig/lisp/types"
)

func Pr_list(lst []types.MalType, pr bool,
	start string, end string, join string) string {
	str_list := make([]string, 0, len(lst))
	for _, e := range lst {
		str_list = append(str_list, Pr_str(e, pr))
	}
	return start + strings.Join(str_list, join) + end
}

func Pr_str(obj types.MalType, print_readably bool) string {
	switch tobj := obj.(type) {
	case types.LispPrintable:
		return tobj.LispPrint(Pr_str)
	// case lisperror.LispError:
	// 	return tobj.LispPrint(Pr_str)
	case types.List:
		return Pr_list(tobj.Val, print_readably, "(", ")", " ")
	case types.Vector:
		return Pr_list(tobj.Val, print_readably, "[", "]", " ")
	case marshaler.HashMap:
		value, err := tobj.MarshalHashMap()
		if err != nil {
			return "{}"
		}
		return hashMapToString(value.(types.HashMap), print_readably)
	case types.HashMap:
		return hashMapToString(tobj, print_readably)
	case types.Set:
		str_list := make([]string, 0, len(tobj.Val))
		for k := range tobj.Val {
			str_list = append(str_list, Pr_str(k, print_readably))
		}
		return "#{" + strings.Join(str_list, " ") + "}"
	case string:
		if strings.HasPrefix(tobj, "\u029e") {
			return ":" + tobj[2:]
		} else if print_readably {
			if strings.HasPrefix(tobj, `{"`) && strings.HasSuffix(tobj, `}`) {
				return `¬` + strings.Replace(tobj, `¬`, `¬¬`, -1) + `¬`
			} else {
				return `"` + strings.Replace(
					strings.Replace(
						strings.Replace(tobj, `\`, `\\`, -1),
						`"`, `\"`, -1),
					"\n", `\n`, -1) + `"`
			}
		} else {
			return tobj
		}
	case types.Symbol:
		return tobj.Val
	case nil:
		return "nil"
	case types.MalFunc:
		return "(fn " +
			Pr_str(tobj.Params, true) + " " +
			Pr_str(tobj.Exp, true) + ")"
	case types.Func:
		return fmt.Sprintf("«function %v»", strings.ToLower(runtime.FuncForPC(reflect.ValueOf(tobj.Fn).Pointer()).Name()))
	case func([]types.MalType) (types.MalType, error):
		return fmt.Sprintf("«function %v»", obj)
	case error:
		return "«go-error " + Pr_str(tobj.Error(), true) + "»"
	// case error:
	// 	return Pr_str(tobj.Error(), true)
	// case *types.Atom:
	// 	return "(atom " +
	// 		Pr_str(tobj.Val, true) + ")"
	default:
		return fmt.Sprintf("%v", obj)
	}
}

func hashMapToString(tobj types.HashMap, print_readably bool) string {
	str_list := make([]string, 0, len(tobj.Val)*2)
	for k, v := range tobj.Val {
		str_list = append(str_list, Pr_str(k, print_readably))
		str_list = append(str_list, Pr_str(v, print_readably))
	}
	return "{" + strings.Join(str_list, " ") + "}"
}
