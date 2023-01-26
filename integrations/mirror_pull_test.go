// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"context"
	"strconv"
	"net/http"
	"net/url"
	"os"
	"testing"

	"code.gitea.io/gitea/models"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/migration"
	"code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/util"
	mirror_service "code.gitea.io/gitea/services/mirror"
	release_service "code.gitea.io/gitea/services/release"

	"github.com/stretchr/testify/assert"
)

func TestMirrorPull(t *testing.T) {
	defer prepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}).(*repo_model.Repository)
	repoPath := repo_model.RepoPath(user.Name, repo.Name)

	opts := migration.MigrateOptions{
		RepoName:    "test_mirror",
		Description: "Test mirror",
		Private:     false,
		Mirror:      true,
		CloneAddr:   repoPath,
		Wiki:        true,
		Releases:    false,
	}

	mirrorRepo, err := repository.CreateRepository(user, user, models.CreateRepoOptions{
		Name:        opts.RepoName,
		Description: opts.Description,
		IsPrivate:   opts.Private,
		IsMirror:    opts.Mirror,
		Status:      repo_model.RepositoryBeingMigrated,
	})
	assert.NoError(t, err)

	ctx := context.Background()

	mirror, err := repository.MigrateRepositoryGitData(ctx, user, mirrorRepo, opts, nil)
	assert.NoError(t, err)

	gitRepo, err := git.OpenRepository(repoPath)
	assert.NoError(t, err)
	defer gitRepo.Close()

	findOptions := models.FindReleasesOptions{IncludeDrafts: true, IncludeTags: true}
	initCount, err := models.GetReleaseCountByRepoID(mirror.ID, findOptions)
	assert.NoError(t, err)

	assert.NoError(t, release_service.CreateRelease(gitRepo, &models.Release{
		RepoID:       repo.ID,
		Repo:         repo,
		PublisherID:  user.ID,
		Publisher:    user,
		TagName:      "v0.2",
		Target:       "master",
		Title:        "v0.2 is released",
		Note:         "v0.2 is released",
		IsDraft:      false,
		IsPrerelease: false,
		IsTag:        true,
	}, nil, ""))

	_, err = repo_model.GetMirrorByRepoID(mirror.ID)
	assert.NoError(t, err)

	ok := mirror_service.SyncPullMirror(ctx, mirror.ID)
	assert.True(t, ok)

	count, err := models.GetReleaseCountByRepoID(mirror.ID, findOptions)
	assert.NoError(t, err)
	assert.EqualValues(t, initCount+1, count)

	release, err := models.GetRelease(repo.ID, "v0.2")
	assert.NoError(t, err)
	assert.NoError(t, release_service.DeleteReleaseByID(release.ID, user, true))

	ok = mirror_service.SyncPullMirror(ctx, mirror.ID)
	assert.True(t, ok)

	count, err = models.GetReleaseCountByRepoID(mirror.ID, findOptions)
	assert.NoError(t, err)
	assert.EqualValues(t, initCount, count)
}

func TestMirrorPullQuota(t *testing.T) {
	defer prepareTestEnv(t)()
	onGiteaRun(t, testMirrorPull)
}

func testMirrorPull(t *testing.T, u *url.URL) {
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}).(*repo_model.Repository)
	repoPath := repo_model.RepoPath(user.Name, repo.Name)

	opts := migration.MigrateOptions{
		RepoName:    "test_mirror",
		Description: "Test mirror",
		Private:     false,
		Mirror:      true,
		CloneAddr:   repoPath,
		Wiki:        true,
		Releases:    false,
	}

	mirrorRepo, err := repository.CreateRepository(user, user, models.CreateRepoOptions{
		Name:        opts.RepoName,
		Description: opts.Description,
		IsPrivate:   opts.Private,
		IsMirror:    opts.Mirror,
		Status:      repo_model.RepositoryBeingMigrated,
	})
	assert.NoError(t, err)

	ctx := context.Background()

	usedSpace := getUsedSpaceMoreThan(t, 0, user.ID)
	mirror, err := repository.MigrateRepositoryGitData(ctx, user, mirrorRepo, opts, nil)
	assert.NoError(t, err)
	_, err = repo_model.GetMirrorByRepoID(mirror.ID)
	assert.NoError(t, err)
	usedSpace = getUsedSpaceMoreThan(t, usedSpace, user.ID)

	commitSizeKb := 10

	httpContext := NewAPITestContext(t, user.Name, repo.Name)
	u.Path = httpContext.GitPath()
	u.User = url.UserPassword(user.Name, userPassword)

	dstPath, err := os.MkdirTemp("", httpContext.Reponame)
	assert.NoError(t, err)
	defer util.RemoveAll(dstPath)

	t.Run("Prepare by clone", doGitClone(dstPath, u))

	t.Run("Increase Quota On Normal", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		t.Run("Simple commit", func(t *testing.T) {
			defer PrintCurrentTest(t)()
			doCommitAndPush(t, commitSizeKb*1024, dstPath, "data-file-", false)
			usedSpace = getUsedSpaceMoreThan(t, usedSpace, user.ID)

			ok := mirror_service.SyncPullMirror(ctx, mirror.ID)
			assert.True(t, ok)
			usedSpace = getUsedSpaceMoreThan(t, usedSpace-1+int64(commitSizeKb), user.ID)
		})

		t.Run("LFS", func(t *testing.T) {
			lfsCommitAndPushTest(t, dstPath, false)
			usedSpace = getUsedSpaceMoreThan(t, usedSpace, user.ID)

			ok := mirror_service.SyncPullMirror(ctx, mirror.ID)
			assert.True(t, ok)
			usedSpace = getUsedSpaceMoreThan(t, usedSpace, user.ID)
		})

		t.Run("Wiki", func(t *testing.T) {
			defer PrintCurrentTest(t)()

			session := loginUser(t, user.Name)
			token := getTokenForLoggedInUser(t, session)
			createWikiPage(t, session, token, user.Name, repo.Name, "wikki", http.StatusCreated)
			usedSpace = getUsedSpaceMoreThan(t, usedSpace, user.ID)

			ok := mirror_service.SyncPullMirror(ctx, mirror.ID)
			assert.True(t, ok)
			usedSpace = getUsedSpaceMoreThan(t, usedSpace, user.ID)
		})
	})

	t.Run("Increase Quota On Fail", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		for i := 0; i < 20; i++ {
			doCommitAndPush(t, commitSizeKb*1024, dstPath, "data-file-"+strconv.Itoa(i), false)
			_, err := generateCommitWithNewData(commitSizeKb*1024, dstPath, "user2@example.com", "User Two", "data-file-"+strconv.Itoa(i))
			assert.NoError(t, err)
		}
		_, err = git.NewCommand("push", "origin", "master").RunInDir(dstPath)
		assert.NoError(t, err)
		usedSpace = getUsedSpaceMoreThan(t, usedSpace, user.ID)

		forceChangeQuota(user.ID, usedSpace+int64(commitSizeKb)*2)

		ok := mirror_service.SyncPullMirror(ctx, mirror.ID)
		assert.False(t, ok)
		usedSpace = getUsedSpaceMoreThan(t, usedSpace, user.ID)
	})
}
