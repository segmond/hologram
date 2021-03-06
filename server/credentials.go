// Copyright 2014 AdRoll, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package server

// TODO make this not a service, instead just exposing a couple of functions on a struct, which is eminently mockable.
// It was a service before because it held state, which is now gone.

import (
	"errors"
	"fmt"
	"github.com/AdRoll/hologram/log"
	"github.com/aws/aws-sdk-go/service/sts"
	"strings"
)

/*
CredentialService implements workflows that return temporary
credentials to calling processes. No caching is done of these
results other than that which the CredentialService does itself.
*/
type CredentialService interface {
	AssumeRole(user *User, role string, enableLDAPRoles bool) (*sts.Credentials, error)
}

/*
STSImplementation exists to enable dependency injection of an
implementation of STS.
*/
type STSImplementation interface {
	AssumeRole(options *sts.AssumeRoleInput) (*sts.AssumeRoleOutput, error)
}

/*
directSessionTokenService is a credential workflow that speaks to AWS STS
directly. It will always return long-lived credentials the developer account
compiled into the application.
*/
type directSessionTokenService struct {
	iamAccount string
	sts        *sts.STS
}

/*
NewDirectSessionTokenService returns a credential service that talks
to Amazon directly.
*/
func NewDirectSessionTokenService(iamAccount string, sts *sts.STS) *directSessionTokenService {
	return &directSessionTokenService{iamAccount: iamAccount, sts: sts}
}

func (s *directSessionTokenService) Start() error {
	return nil
}

func (s *directSessionTokenService) buildARN(role string) string {
	var arn string

	if strings.HasPrefix(role, "arn:aws:iam") {
		arn = role
	} else if strings.Contains(role, ":role/") {
		arn = fmt.Sprintf("arn:aws:iam::%s", role)
	} else {
		arn = fmt.Sprintf("arn:aws:iam::%s:role/%s", s.iamAccount, role)
	}

	return arn
}

func (s *directSessionTokenService) AssumeRole(user *User, role string, enableLDAPRoles bool) (*sts.Credentials, error) {
	var arn string = s.buildARN(role)

	log.Debug("Checking ARN %s against user %s (with access %s)", arn, user.Username, user.ARNs)

	if enableLDAPRoles {
		found := false
		for _, a := range user.ARNs {
			a = s.buildARN(a)
			if arn == a {
				found = true
				break
			}
		}

		log.Debug("Found %s", found)

		if !found {
			return nil, errors.New(fmt.Sprintf("User %s is not authorized to assume role %s!", user.Username, arn))
		}
	}
	log.Debug("User: %s", user.Username)
	duration := int64(3600)
	options := &sts.AssumeRoleInput{
		DurationSeconds: &duration,
		RoleArn:         &arn,
		RoleSessionName: &user.Username,
	}

	r, err := s.sts.AssumeRole(options)
	if err != nil {
		log.Debug("Error!! %s", err.Error())
		return nil, err
	}
	return r.Credentials, nil
}
