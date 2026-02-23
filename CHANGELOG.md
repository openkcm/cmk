# Changelog

## [0.3.0](https://github.com/openkcm/cmk/compare/v0.2.1...v0.3.0) (2026-02-23)


### Features

* add component-specific resource overrides for deployments ([#82](https://github.com/openkcm/cmk/issues/82)) ([88f31f8](https://github.com/openkcm/cmk/commit/88f31f8487b69f9f0a0ee7d0310a3f4c37a6b1e7))
* create separate component for event reconciler ([#104](https://github.com/openkcm/cmk/issues/104)) ([5ad0d66](https://github.com/openkcm/cmk/commit/5ad0d66165350ade416d023a8cdbb59d59a7e611))
* enable event reconciler by default in values.yaml ([#122](https://github.com/openkcm/cmk/issues/122)) ([54bd66e](https://github.com/openkcm/cmk/commit/54bd66eb5602cbe4f3a6ace178c79fa41a87d0df))
* system and workflow pkey check ([#66](https://github.com/openkcm/cmk/issues/66)) ([8da013f](https://github.com/openkcm/cmk/commit/8da013fe1cb8706cc075d3fa3bda829861e80ae3))
* update tenant info ([#102](https://github.com/openkcm/cmk/issues/102)) ([0095366](https://github.com/openkcm/cmk/commit/0095366fd9a8e353322f94892c3aa47588051333))
* Update workflow email ([#54](https://github.com/openkcm/cmk/issues/54)) ([dc00b93](https://github.com/openkcm/cmk/commit/dc00b93b709635ab3571722c127b049456707e30))
* workflow settings configurable ([#56](https://github.com/openkcm/cmk/issues/56)) ([1684142](https://github.com/openkcm/cmk/commit/1684142bde3c9c0bfd4ef20d0b2e755a37219c6a))


### Bug Fixes

* allow unlink when system in failed state ([#129](https://github.com/openkcm/cmk/issues/129)) ([b0f1e6a](https://github.com/openkcm/cmk/commit/b0f1e6a9e62fdb232665f853177490966af9f1eb))
* auditor readonly all keyconfigs ([#100](https://github.com/openkcm/cmk/issues/100)) ([746fbd8](https://github.com/openkcm/cmk/commit/746fbd80081f7504f173b6ebcca3f4c17248b9fd))
* **deps:** update plugin-sdk version to v0.9.5  ([#137](https://github.com/openkcm/cmk/issues/137)) ([7245ba9](https://github.com/openkcm/cmk/commit/7245ba980588449a32a8ef20061740f8b9e60462))
* include the pull-requests: read into workflow permission ([#117](https://github.com/openkcm/cmk/issues/117)) ([66d4765](https://github.com/openkcm/cmk/commit/66d4765b8cca2d76094f9324094b84b154503498))
* keyconfig cert returning exists by default ([#112](https://github.com/openkcm/cmk/issues/112)) ([94fc75c](https://github.com/openkcm/cmk/commit/94fc75cc1779e800ef0778662cb0e13631eb28ba))
* keyconfig count ([#79](https://github.com/openkcm/cmk/issues/79)) ([6ef65e1](https://github.com/openkcm/cmk/commit/6ef65e16e8d273d6b14bbd29400fe022d83ec8f8))
* linter pre-alloc errors ([#108](https://github.com/openkcm/cmk/issues/108)) ([20bbd8f](https://github.com/openkcm/cmk/commit/20bbd8fc73912749977415329a7698b922f76ef5))
* listing duplicated workflow tasks ([#121](https://github.com/openkcm/cmk/issues/121)) ([dd362ab](https://github.com/openkcm/cmk/commit/dd362abf3500dd6d08cd44379ad7538b2e3a1ae9))
* only unmap system from tenant on unlink system action ([#62](https://github.com/openkcm/cmk/issues/62)) ([2304821](https://github.com/openkcm/cmk/commit/2304821fd2cacfe0c639d773046a0183c63ba3ca))
* pagination on system refresh and toLower type ([#140](https://github.com/openkcm/cmk/issues/140)) ([5f7683d](https://github.com/openkcm/cmk/commit/5f7683d3b2558994a2adef493d326f006bd528b8))
* prepare plugins to switch from raw grpc interface -&gt; abstract golang interface ([#123](https://github.com/openkcm/cmk/issues/123)) ([478e97b](https://github.com/openkcm/cmk/commit/478e97b9aa03d94abe7dbef369b37f6c8764b2a4))
* release please configuration ([#73](https://github.com/openkcm/cmk/issues/73)) ([66c5836](https://github.com/openkcm/cmk/commit/66c5836128f75143eb061048f206c1c8b72d22a1))
* removed unused/dead code ([#115](https://github.com/openkcm/cmk/issues/115)) ([423849b](https://github.com/openkcm/cmk/commit/423849b6750067d763bebb05b1f51a0411a97d08))
* rotate certs in batch ([#90](https://github.com/openkcm/cmk/issues/90)) ([4ebea43](https://github.com/openkcm/cmk/commit/4ebea43b3311e911c3348e676f86a8b84ce4af97))
* system information switch to golang interfaces ([#124](https://github.com/openkcm/cmk/issues/124)) ([3d5389b](https://github.com/openkcm/cmk/commit/3d5389b0dad03c1b34dbc7497af1fed8f2a83259))
* system type must be lowercase for registry ([#61](https://github.com/openkcm/cmk/issues/61)) ([627bb71](https://github.com/openkcm/cmk/commit/627bb71a1159e350d5de659a9407810e4cd5c1e5))
* Unable to Switch Primary keys , while connected to System ([#58](https://github.com/openkcm/cmk/issues/58)) ([a00a23c](https://github.com/openkcm/cmk/commit/a00a23cf289a19eb61804f40dbbbed4c1a5fe996))
* unmap system only run on tenant termination system unlink ([#64](https://github.com/openkcm/cmk/issues/64)) ([1f3cd91](https://github.com/openkcm/cmk/commit/1f3cd9193bb2f07902390ad7b6b3ea63ccb04452))
* update keystores endpoint resource type ([#107](https://github.com/openkcm/cmk/issues/107)) ([8b448ff](https://github.com/openkcm/cmk/commit/8b448ff9a7fb196db86f1dec25b93e785f3ee833))
* update plugin-sdk to v0.9.6 ([#139](https://github.com/openkcm/cmk/issues/139)) ([4b95d1e](https://github.com/openkcm/cmk/commit/4b95d1e86e0b865f9c39b3c79320f0a3942d9591))
* upgrade the plugin-sdk version to v0.9.2 ([#116](https://github.com/openkcm/cmk/issues/116)) ([5bb8d9b](https://github.com/openkcm/cmk/commit/5bb8d9b78e1d57b827b7300bc244825a340adf13))
* use common-sdk status serve that cover default checks ([#111](https://github.com/openkcm/cmk/issues/111)) ([d143b29](https://github.com/openkcm/cmk/commit/d143b29324e5996307a52d0e8ee50476e0021925))
* verify name on creation and white space validation ([#93](https://github.com/openkcm/cmk/issues/93)) ([a9e680b](https://github.com/openkcm/cmk/commit/a9e680b4e7aa22d29c4c9e12f9b5c9c933b51a0f))
* workflow expiry task ([#76](https://github.com/openkcm/cmk/issues/76)) ([9af4390](https://github.com/openkcm/cmk/commit/9af4390f4ca304b1290dcbbb9de30b8135493626))

## [0.2.1](https://github.com/openkcm/cmk/compare/v0.2.0...v0.2.1) (2026-02-05)


### Bug Fixes

* remove blocking on terminate tenant ([#59](https://github.com/openkcm/cmk/issues/59)) ([11c4fcd](https://github.com/openkcm/cmk/commit/11c4fcdac39c3890d964eacccbd17899d51876e5))

## [0.2.0](https://github.com/openkcm/cmk/compare/v0.1.2...v0.2.0) (2026-02-04)


### Features

* terminate tenant mapping ([#48](https://github.com/openkcm/cmk/issues/48)) ([5c5fb6a](https://github.com/openkcm/cmk/commit/5c5fb6a9d833beefb1b90d25462c5789c34ab84a))

## [0.1.2](https://github.com/openkcm/cmk/compare/v0.1.1...v0.1.2) (2026-02-04)


### Bug Fixes

* skip validations tests ([#49](https://github.com/openkcm/cmk/issues/49)) ([8e2bbdc](https://github.com/openkcm/cmk/commit/8e2bbdc4b5fc3720dc4dc3d151f96aaefcd0a259))

## [0.1.1](https://github.com/openkcm/cmk/compare/v0.1.0...v0.1.1) (2026-02-04)


### Bug Fixes

* have chart into a separate folder; chnages on the Taskfile.yaml ([#45](https://github.com/openkcm/cmk/issues/45)) ([f3468fd](https://github.com/openkcm/cmk/commit/f3468fdb921cd7e28146373213bc709943a900bc))

## [0.1.0](https://github.com/openkcm/cmk/compare/v0.0.1...v0.1.0) (2026-02-04)


### Features

* add builtin plugins doing nothing at this moment ([#12](https://github.com/openkcm/cmk/issues/12)) ([5f25603](https://github.com/openkcm/cmk/commit/5f25603cc47d03c8dbba62a61f484c1987e70589))
* cmk api backend and other suite of applications for cmk([#6](https://github.com/openkcm/cmk/issues/6)) ([8ea13c6](https://github.com/openkcm/cmk/commit/8ea13c6d77473b081a394341e790352b5988a97d))


### Bug Fixes

* bunch of many other updated code ([#36](https://github.com/openkcm/cmk/issues/36)) ([0bf2ee0](https://github.com/openkcm/cmk/commit/0bf2ee01ed95d4b10acc4b8660b7fc32913707c4))
* **deps:** bump github.com/aws/aws-sdk-go-v2 from 1.36.5 to 1.39.2 ([#7](https://github.com/openkcm/cmk/issues/7)) ([09924a4](https://github.com/openkcm/cmk/commit/09924a40e459527175796fc022f359642e43ae13))
* **deps:** bump github.com/getkin/kin-openapi from 0.132.0 to 0.133.0 ([#8](https://github.com/openkcm/cmk/issues/8)) ([8d45930](https://github.com/openkcm/cmk/commit/8d459307fe650a8f4964bbdab8a78e615f03db12))
* **deps:** bump github.com/testcontainers/testcontainers-go from 0.38.0 to 0.39.0 ([#9](https://github.com/openkcm/cmk/issues/9)) ([49d3caa](https://github.com/openkcm/cmk/commit/49d3caa54518f3568901d4b8dd22331326f33a24))
* makefile test script exit code on the failures ([#14](https://github.com/openkcm/cmk/issues/14)) ([b41792f](https://github.com/openkcm/cmk/commit/b41792f397adcd7e7ff099d280dd7c4e5949c6dc))
* renamed the repo name ([929205d](https://github.com/openkcm/cmk/commit/929205d5f7867afb72e28e7f456122e5342d07c9))
* run tests in parallel ([#16](https://github.com/openkcm/cmk/issues/16)) ([0aa53af](https://github.com/openkcm/cmk/commit/0aa53af6b5bb8a09304d069af0fc308504919601))
* set --rerun-fails on max 5 rounds of retry ([#17](https://github.com/openkcm/cmk/issues/17)) ([93249f9](https://github.com/openkcm/cmk/commit/93249f9c31e564bbc0df086e4d3097d541b6eb3c))
* Sync current CMK state to openkcm ([#39](https://github.com/openkcm/cmk/issues/39)) ([a98d1b7](https://github.com/openkcm/cmk/commit/a98d1b7585b97ce929f6c4eaf85ab6a3cbe81095))
* test makefile action ([#15](https://github.com/openkcm/cmk/issues/15)) ([1237d44](https://github.com/openkcm/cmk/commit/1237d44e4d564f8dc0f11e8b063a71a8213f6b54))
* test manifest command ([#18](https://github.com/openkcm/cmk/issues/18)) ([1a874af](https://github.com/openkcm/cmk/commit/1a874af1e035ff0e3e4d994642c916d4f8fd675a))
* tests commands on the Makefile ([#13](https://github.com/openkcm/cmk/issues/13)) ([4c3f50c](https://github.com/openkcm/cmk/commit/4c3f50c7a637db84996987cb2a7fad04b21398ca))

## 0.0.1 (2025-10-10)


### Bug Fixes

* add all files for workflows ([#4](https://github.com/openkcm/cmk/issues/4)) ([013fc37](https://github.com/openkcm/cmk/commit/013fc3788665f6491c2bb0b5ce978a58b5863e9d))
* set base versioning files ([a283c32](https://github.com/openkcm/cmk/commit/a283c32ec8cb84acc343f124edfc9d275de1c665))


### Miscellaneous Chores

* reset version to 0.0.1 ([bbe38ef](https://github.com/openkcm/cmk/commit/bbe38ef38ae5b81ca324161f3bfccb75e1352deb))
