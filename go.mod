module github.com/iszk1215/mora

go 1.18

require (
	github.com/deckarep/golang-set/v2 v2.1.0
	github.com/drone/drone v1.10.1
	github.com/drone/go-login v1.1.0
	github.com/drone/go-scm v1.23.0
	github.com/elliotchance/pie/v2 v2.0.1
	github.com/go-chi/chi/v5 v5.0.7
	github.com/golang/mock v1.3.1
	github.com/jmoiron/sqlx v0.0.0-20180614180643-0dae4fefe7c0
	github.com/mattn/go-sqlite3 v1.14.14
	github.com/rs/zerolog v1.26.1
	github.com/stretchr/testify v1.7.1
	golang.org/x/tools v0.1.11
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/google/go-cmp v0.5.7 // indirect
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/exp v0.0.0-20220321173239-a90fa8a75705 // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
)

replace github.com/h2non/gock => gopkg.in/h2non/gock.v1 v1.0.15
