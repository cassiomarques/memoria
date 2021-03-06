(defproject memoria "0.1.0-SNAPSHOT"
  :description "A webapp to save things that you would otherwise forget."
  :url "http://example.com/FIXME"
  :license {:name "Eclipse Public License"
            :url "http://www.eclipse.org/legal/epl-v10.html"}

  :dependencies [[org.clojure/clojure "1.7.0"]
                 [org.clojure/clojurescript "1.7.48"]
                 [yesql "0.5.0-rc3"]
                 [hikari-cp "1.2.4"]
                 [ragtime "0.5.0"]
                 [org.postgresql/postgresql "9.4-1201-jdbc41"]
                 [compojure "1.4.0"]
                 [ring "1.4.0"]
                 [ring/ring-json "0.3.1"]
                 [org.clojure/data.json "0.2.6"]
                 [clj-http "2.0.0"]
                 [com.taoensso/timbre "4.0.2"]
                 [slingshot "0.12.2"]
                 [bouncer "0.3.3"]]

  :plugins [[lein-ring "0.9.6"]
            [lein-environ "1.0.0"]
            [lein-figwheel "0.3.9"]
            [lein-cljsbuild "1.1.0"]]

  :source-paths ["src/clj" "src/cljs"]
  :repl-options {:init  (require '[memoria.repl :refer :all])}
  :target-path "target/%s"

  :profiles {:uberjar {:aot :all}
             :dev {:dependencies [[ring/ring-mock "0.2.0"]]}}

  :cljsbuild {:builds
              [{;; CLJS source code path
                :source-paths ["src/cljs"]

                ;; Google Closure (CLS) options configuration
                :compiler {;; CLS generated JS script filename
                           :output-to "resources/public/js/memoria.js"

                           ;; minimal JS optimization directive
                           :optimizations :whitespace

                           ;; generated JS code prettyfication
                           :pretty-print true}}]}

  :ring  {:handler memoria.core/app})
