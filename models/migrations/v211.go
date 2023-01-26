// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"

	"code.gitea.io/gitea/models/user"

	"xorm.io/xorm"
)

func добавьПоляДляУчётаКвотПоМестам211(x *xorm.Engine) error {
	type User struct {
		ID        int64  `xorm:"pk autoincr"`
		LowerName string `xorm:"UNIQUE NOT NULL"`
		Name      string `xorm:"UNIQUE NOT NULL"`
		FullName  string
		// Email is the primary email address (to be used for communication)
		Email                        string `xorm:"NOT NULL"`
		KeepEmailPrivate             bool
		EmailNotificationsPreference string `xorm:"VARCHAR(20) NOT NULL DEFAULT 'enabled'"`
		Passwd                       string `xorm:"NOT NULL"`
		PasswdHashAlgo               string `xorm:"NOT NULL DEFAULT 'argon2'"`

		// MustChangePassword is an attribute that determines if a user
		// is to change his/her password after registration.
		MustChangePassword bool `xorm:"NOT NULL DEFAULT false"`

		LoginType   auth.Type
		LoginSource int64 `xorm:"NOT NULL DEFAULT 0"`
		LoginName   string
		Type        user.UserType
		Location    string
		Website     string
		Rands       string `xorm:"VARCHAR(32)"`
		Salt        string `xorm:"VARCHAR(32)"`
		Language    string `xorm:"VARCHAR(5)"`
		Description string

		CreatedUnix   timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix   timeutil.TimeStamp `xorm:"INDEX updated"`
		LastLoginUnix timeutil.TimeStamp `xorm:"INDEX"`

		// Remember visibility choice for convenience, true for private
		LastRepoVisibility bool
		// Maximum repository creation limit, -1 means use global default
		MaxRepoCreation int `xorm:"NOT NULL DEFAULT -1"`

		// IsActive true: primary email is activated, user can access Web UI and Git SSH.
		// false: an inactive user can only log in Web UI for account operations (ex: activate the account by email), no other access.
		IsActive bool `xorm:"INDEX"`
		// the user is a Gitea admin, who can access all repositories and the admin pages.
		IsAdmin bool
		// true: the user is only allowed to see organizations/repositories that they has explicit rights to.
		// (ex: in private Gitea instances user won't be allowed to see even organizations/repositories that are set as public)
		IsRestricted bool `xorm:"NOT NULL DEFAULT false"`

		AllowGitHook            bool
		AllowImportLocal        bool // Allow migrate repository by local path
		AllowCreateOrganization bool `xorm:"DEFAULT true"`

		// true: the user is not allowed to log in Web UI. Git/SSH access could still be allowed (please refer to Git/SSH access related code/documents)
		ProhibitLogin bool `xorm:"NOT NULL DEFAULT false"`

		// Avatar
		Avatar          string `xorm:"VARCHAR(2048) NOT NULL"`
		AvatarEmail     string `xorm:"NOT NULL"`
		UseCustomAvatar bool

		// Counters
		NumFollowers int
		NumFollowing int `xorm:"NOT NULL DEFAULT 0"`
		NumStars     int
		NumRepos     int

		// For organization
		NumTeams                  int
		NumMembers                int
		Visibility                structs.VisibleType `xorm:"NOT NULL DEFAULT 0"`
		RepoAdminChangeTeamAccess bool                `xorm:"NOT NULL DEFAULT false"`

		// Preferences
		DiffViewStyle       string `xorm:"NOT NULL DEFAULT ''"`
		Theme               string `xorm:"NOT NULL DEFAULT ''"`
		KeepActivityPrivate bool   `xorm:"NOT NULL DEFAULT false"`

		// Ограничение пространства
		QuotaKb     int64 `xorm:"NOT NULL DEFAULT 10000"` // 0 - нет ограничений
		SpaceUsedKb int64 `xorm:"NOT NULL DEFAULT 18"`
	}

	if err := x.Sync2(&User{}); err != nil {
		return err
	}

	return nil
}
