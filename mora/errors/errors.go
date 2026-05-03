// Copyright 2019 Drone IO, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package errors

var (
	// ErrInvalidToken is returned when the api request token is invalid.
	ErrInvalidToken = New("Invalid or missing token")

	// ErrUnauthorized is returned when the user is not authorized.
	ErrUnauthorized = New("Unauthorized")

	// ErrForbidden is returned when user access is forbidden.
	ErrForbidden = New("Forbidden")

	// ErrNotFound is returned when a resource is not found.
	ErrNotFound = New("Not Found")

	// ErrNotImplemented is returned when an endpoint is not implemented.
	ErrNotImplemented = New("Not Implemented")

	// ErrTokenNotFound is returned when a token is not found in a session.
	ErrTokenNotFound = New("token not found in a session")

	// ErrRepositoryNotFound is returned when a repository is not found.
	ErrRepositoryNotFound = New("no repository found")

	// ErrMetricNotFound is returned when a metric is not found.
	ErrMetricNotFound = New("no metric found")

	// ErrMetricInUse is returned when a metric is in use.
	ErrMetricInUse = New("metric in use")

	// ErrItemNotFound is returned when an item is not found.
	ErrItemNotFound = New("no item found")

	// ErrItemInUse is returned when an item is in use.
	ErrItemInUse = New("item in use")
)

// Error represents a json-encoded API error.
type Error struct {
	Message string `json:"message"`
}

func (e *Error) Error() string {
	return e.Message
}

// New returns a new error message.
func New(text string) error {
	return &Error{Message: text}
}
