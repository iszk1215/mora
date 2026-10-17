// @title          Mora API
// @version        1.0
// @description    Mora is a tracker API.
// @BasePath       /
//
// @tag.name         tracker
// @tag.description  Trackers, series, and values
// @tag.name         coverage
// @tag.description  Code coverage upload and retrieval
// @tag.name         udm
// @tag.description  User-defined metrics
// @tag.name         user
// @tag.description  Users, profiles, and API keys
// @tag.name         auth
// @tag.description  Authentication and sign-up
// @tag.name         server
// @tag.description  Server configuration and repositories

package main

import "github.com/iszk1215/mora/cmd"

func main() {
	cmd.Execute()
}
