# Releasing

This guide explains how to publish the provider to the
[Terraform Registry](https://registry.terraform.io/) and the
[OpenTofu Registry](https://search.opentofu.org/). Both registries read the provider from GitHub
releases of this repository. A release is created automatically when you push a version tag.

## How a release works

1. You push a tag such as `v0.1.0`.
2. The [release workflow](.github/workflows/release.yml) runs [GoReleaser](https://goreleaser.com/)
   with [`.goreleaser.yml`](.goreleaser.yml). GoReleaser builds the provider for every supported
   operating system and processor, creates a `SHA256SUMS` file, and signs that file with the
   release GPG key.
3. GoReleaser uploads everything to a GitHub release.
4. The Terraform Registry and the OpenTofu Registry notice the new release and publish it. They
   check the signature against the public key that you registered with them.

Each registry also shows the documentation in `docs/` from the tagged commit.

## The release signing key

Both registries need a GPG key to check the signatures. The key must be RSA, because the Terraform
Registry does not accept ECC keys.

A key already exists for this repository:

* Fingerprint: `EC973EEC56F04F2C92EE43D953D83AB409F9A781`
* Type: RSA. The same key signs the maintainer's terraform-provider-contabo releases.
* User ID: `hayone1 (terraform-provider-contabo releases)`
* Revocation certificate: `.secrets/release-signing-key.revocation.rev`

The private key and its passphrase are in the maintainer's git-ignored `.secrets/` folder, in the `.env` file (`GPG_PRIVATE_KEY`,
`PASSPHRASE` and `GPG_FINGERPRINT`) and in the maintainer's local GPG keyring. Keep a backup of
`.env` in a password manager. If you lose the private key, you must create a new key and register
it with both registries again.

Print the public key at any time with:

```shell
task release:public-key
```

## First release

Do these steps once, in this order.

### 1. Create the GitHub repository

The repository must be public, and its name must be `terraform-provider-wso2is`.

```shell
git add -A
git commit -m "feat: first release of the WSO2 Identity Server provider"
gh repo create hayone1/terraform-provider-wso2is --public --source=. --push
```

Before you commit, check that `git status` does not list `.env` or anything under `_debug/`.
Both are in `.gitignore`.

### 2. Give the release workflow the signing key

```shell
task release:secrets
```

This command uploads `GPG_PRIVATE_KEY` and `PASSPHRASE` from `.env` to the repository's GitHub
Actions secrets.

Optional: the acceptance workflow pulls the WSO2 image from Docker Hub. To avoid Docker Hub's
limit on anonymous pulls, add the `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` secrets too.

### 3. Wait for the checks to pass

Open the repository's **Actions** tab. Wait until the **CI** and **Acceptance** workflows pass on
the `main` branch.

### 4. Register the key with the Terraform Registry

This key already signs terraform-provider-contabo. If you added it to the `hayone1` namespace
for that provider, skip this step.

1. Go to <https://registry.terraform.io/> and sign in with your GitHub account.
2. Open **User Settings > Signing Keys**.
3. Select the `hayone1` namespace, and paste the output of `task release:public-key`.
4. Save the key.

### 5. Register the key and the provider with the OpenTofu Registry

The OpenTofu Registry only accepts submissions through its GitHub issue forms in a web browser. It
closes submissions that come from the `gh` CLI or the GitHub API.

1. Open
   [Submit new Provider Signing Key](https://github.com/opentofu/registry/issues/new?assignees=&labels=provider-key%2Csubmission&projects=&template=provider_key.yml&title=Provider+Key%3A+).
   * **Provider Namespace:** `hayone1`
   * **Provider Name:** `wso2is` (skip this form if the key was added for the whole `hayone1`
     namespace when you published terraform-provider-contabo)
   * **Public Membership:** tick the box. (`hayone1` is your user account, so you are its owner.)
   * **Provider GPG Key:** paste the output of `task release:public-key`.
2. Open
   [Submit new Provider](https://github.com/opentofu/registry/issues/new?assignees=&labels=provider%2Csubmission&projects=&template=provider.yml&title=Provider%3A+).
   * **Provider Repository:** `hayone1/terraform-provider-wso2is`
3. Wait for the OpenTofu team to approve both issues. A bot comments on each issue when it is
   done.

You can do step 5 before or after the first tag. The OpenTofu Registry picks up existing releases
when it adds the provider.

### 6. Tag the first release

1. In `CHANGELOG.md`, replace `(Unreleased)` with today's date.
2. Commit the change, and push the tag:

   ```shell
   git commit -am "chore: release v0.1.0"
   git push
   git tag v0.1.0
   git push origin v0.1.0
   ```

3. Open the **Actions** tab and wait for the **Release** workflow to finish.
4. Open the new release on GitHub. It should have one `.zip` file per platform, and the files
   `terraform-provider-wso2is_0.1.0_SHA256SUMS`, `terraform-provider-wso2is_0.1.0_SHA256SUMS.sig`
   and `terraform-provider-wso2is_0.1.0_manifest.json`.

### 7. Publish on the Terraform Registry

1. On <https://registry.terraform.io/>, choose **Publish > Provider**.
2. Select the `hayone1/terraform-provider-wso2is` repository and confirm.

The registry adds a webhook to the repository. From now on, it publishes every new release by
itself.

### 8. Check both registries

Create an empty folder with this `main.tf` file:

```terraform
terraform {
  required_providers {
    wso2is = {
      source  = "hayone1/wso2is"
      version = "0.1.0"
    }
  }
}
```

Then run `terraform init` and `tofu init` in that folder. Both commands should download the
provider and report that the signature is valid.

## Later releases

1. Make sure CI and the acceptance tests pass on `main`.
2. Add a section for the new version to `CHANGELOG.md`. Use
   [semantic versioning](https://semver.org/): a new major version for breaking changes, a new
   minor version for new features, and a new patch version for bug fixes only.
3. Commit, tag and push, as in step 6 above.

Both registries pick up the new release by themselves. A published version cannot be changed or
removed, so release a new patch version to fix a mistake.

## Test a release build locally

```shell
task release:snapshot
```

This command builds all release archives into `dist/` without signing or publishing them.

## Replace the signing key

Replace the key before it expires, or at once if it leaks.

1. Create a new RSA 4096 key with `gpg --full-generate-key`.
2. Update `GPG_FINGERPRINT`, `PASSPHRASE` and `GPG_PRIVATE_KEY` in `.env`. The value of
   `GPG_PRIVATE_KEY` is `gpg --armor --export-secret-keys <fingerprint> | base64 -w0`.
3. Run `task release:secrets`.
4. Add the new public key in the Terraform Registry, and submit it to the OpenTofu Registry with
   the provider key form.
5. If the old key leaked, revoke it and remove it from both registries.
