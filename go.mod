module github.com/iszk1215/mora

go 1.16

require (
	github.com/drone/drone v1.10.1
	github.com/drone/go-login v1.1.0
	github.com/drone/go-scm v1.23.0
	github.com/go-chi/chi/v5 v5.0.7
	github.com/google/go-cmp v0.5.7 // indirect
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/rs/zerolog v1.26.1
	github.com/stretchr/testify v1.7.1
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
	gopkg.in/yaml.v3 v3.0.1
)

replace github.com/h2non/gock => gopkg.in/h2non/gock.v1 v1.0.15
