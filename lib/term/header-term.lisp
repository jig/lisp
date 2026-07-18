;; $MODULE header-term

(do
  ;; Color and attribute sugar over term-style.
  (defn term-red [s] (term-style s {:fg :red}))
  (defn term-green [s] (term-style s {:fg :green}))
  (defn term-yellow [s] (term-style s {:fg :yellow}))
  (defn term-blue [s] (term-style s {:fg :blue}))
  (defn term-magenta [s] (term-style s {:fg :magenta}))
  (defn term-cyan [s] (term-style s {:fg :cyan}))
  (defn term-gray [s] (term-style s {:fg :gray}))
  (defn term-bold [s] (term-style s {:bold true}))
  (defn term-underline [s] (term-style s {:underline true})))
