// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"path"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/migrations"

	"github.com/stretchr/testify/assert"
)

func TestAPIRepoLFSMigrateLocal(t *testing.T) {
	defer prepareTestEnv(t)()

	oldImportLocalPaths := setting.ImportLocalPaths
	oldAllowLocalNetworks := setting.Migrations.AllowLocalNetworks
	setting.ImportLocalPaths = true
	setting.Migrations.AllowLocalNetworks = true
	assert.NoError(t, migrations.Init())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}).(*user_model.User)
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session)

	usedSpace := getUsedSpaceMoreThan(t, 0, 1)
	req := NewRequestWithJSON(t, "POST", "/api/v1/repos/migrate?token="+token, &api.MigrateRepoOptions{
		CloneAddr:   path.Join(setting.RepoRootPath, "migration/lfs-test.git"),
		RepoOwnerID: user.ID,
		RepoName:    "lfs-test-local",
		LFS:         true,
	})
	resp := MakeRequest(t, req, NoExpectedStatus)
	assert.EqualValues(t, http.StatusCreated, resp.Code)
	getUsedSpaceMoreThan(t, usedSpace+defaultSpaceUsedKb, 1)

	store := lfs.NewContentStore()
	ok, _ := store.Verify(lfs.Pointer{Oid: "fb8f7d8435968c4f82a726a92395be4d16f2f63116caf36c8ad35c60831ab041", Size: 6})
	assert.True(t, ok)
	ok, _ = store.Verify(lfs.Pointer{Oid: "d6f175817f886ec6fbbc1515326465fa96c3bfd54a4ea06cfd6dbbd8340e0152", Size: 6})
	assert.True(t, ok)

	setting.ImportLocalPaths = oldImportLocalPaths
	setting.Migrations.AllowLocalNetworks = oldAllowLocalNetworks
	assert.NoError(t, migrations.Init()) // reset old migration settings
}

func TestAPIRepoLFSMigrateLocalQuotaFail(t *testing.T) {
	defer prepareTestEnv(t)()

	setting.ImportLocalPaths = true
	setting.Migrations.AllowLocalNetworks = true
	assert.NoError(t, migrations.Init())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}).(*user_model.User)
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session)
	gitSize, err := util.GetDirectorySize(path.Join(setting.RepoRootPath, "migration/lfs-test.git"))
	assert.NoError(t, err)
	lfsObjSize, err := util.GetDirectorySize(path.Join(setting.RepoRootPath, "migration/lfs-test.git/"))
	assert.NoError(t, err)

	testCases := []struct {
		repoName       string
		expectedStatus int
		quota          int64
	}{
		{repoName: "lfs-quota-fail-in-clone-process", expectedStatus: http.StatusUnprocessableEntity, quota: gitSize + defaultSpaceUsedKb - lfsObjSize + 1},
		{repoName: "lfs-quota-fail-on-start", expectedStatus: http.StatusForbidden, quota: 1},
	}

	for _, testCase := range testCases {
		forceChangeQuota(1, testCase.quota)
		req := NewRequestWithJSON(t, "POST", "/api/v1/repos/migrate?token="+token, &api.MigrateRepoOptions{
			CloneAddr:   path.Join(setting.RepoRootPath, "migration/lfs-test.git"),
			RepoOwnerID: user.ID,
			RepoName:    testCase.repoName,
			LFS:         true,
		})
		MakeRequest(t, req, testCase.expectedStatus)
	}
}
