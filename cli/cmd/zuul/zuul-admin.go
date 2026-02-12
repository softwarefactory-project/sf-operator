/*
Copyright © 2024 Red Hat

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package zuul

import (
	"strconv"

	sfop "github.com/softwarefactory-project/sf-operator/controllers"
)

// zuul-admin proxy commands.

func CreateAuthToken(env *sfop.SFKubeContext, authConfig string, tenant string, user string, expiry int) string {
	_authConfig := authConfig
	if _authConfig == "" {
		_authConfig = "zuul_client"
	}
	createAuthTokenCmd := []string{
		"zuul-admin",
		"create-auth-token",
		"--auth-config", _authConfig,
		"--tenant", tenant,
		"--user", user,
		"--expires-in", strconv.Itoa(expiry),
	}
	token, _ := env.PodExecBytes("zuul-scheduler-0", "zuul-scheduler", createAuthTokenCmd)
	return token.String()
}
