package nstest

import (
	"github.com/jig/mal"
	. "github.com/jig/mal/types"
)

func Load(repl_env EnvType) error {
	if _, err := mal.REPL(repl_env, `(eval (read-string (str "(do "`+AssertMacros+`" nil)"))))`, nil); err != nil {
		return err
	}
	return nil
}

var AssertMacros = `;; assert macros
(defmacro! assert-true
    (fn* [name expr]
        (list 
            'if (try* expr (catch* err err))
                nil 
                {   :failed true 
                    :name name
                    :expr (str expr)})))

(defmacro! assert-false
    (fn* [name expr]
        (list 
            'if (try* expr (catch* err err))
                {   :failed true 
                    :name name
                    :expr (str expr)}
                nil)))

(defmacro! assert-throws
    (fn* [name expr]
        (let* [failureError {   :failed true 
                                :name (str name)
                                :expr (str expr)}]
        ` + "`" + `(try* 
            (do 
                ~expr 
                ~failureError)
            (catch* err nil)))))

(def! test.suite (fn* [name & assert-cases]
    (if 
        (reduce and true 
            (map 
                (fn* [x] 
                    (if  (not (nil? x))
                        (println "TEST SUITE FAIL" name ">" (get x :name) ">>" (get x :expr))
                        true))
                assert-cases))
        (println "TEST SUITE PASS" name "PASS"))))
`
