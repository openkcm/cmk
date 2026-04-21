# Changelog

## [0.8.0](https://github.com/openkcm/cmk/compare/v0.7.0...v0.8.0) (2026-04-21)


### Features

* add allowBYOK landscape feature gate ([#243](https://github.com/openkcm/cmk/issues/243)) ([7cac3c2](https://github.com/openkcm/cmk/commit/7cac3c2b61faf8108114806058ef9aad1d478be8))
* add key rotate audit log ([#248](https://github.com/openkcm/cmk/issues/248)) ([6828a68](https://github.com/openkcm/cmk/commit/6828a681628d71ee361d44a4d2cd1dd39ecf559a))
* add OpenTelemetry tracing support for database connections ([#238](https://github.com/openkcm/cmk/issues/238)) ([256fe37](https://github.com/openkcm/cmk/commit/256fe37e7aab3af817fe0b088d85793a86a6cfd0))
* Asynq Fanout Mechanism and HYOK Refresh Frequency ([#191](https://github.com/openkcm/cmk/issues/191)) ([6054ef8](https://github.com/openkcm/cmk/commit/6054ef8137ff490962873c2df89d23beb6f14ddc))
* extend system event task data with crypto certificate subject ([#245](https://github.com/openkcm/cmk/issues/245)) ([b3b7fd2](https://github.com/openkcm/cmk/commit/b3b7fd20199ff7753155745175c41d002355d615))
* support HYOK key rotation ([#233](https://github.com/openkcm/cmk/issues/233)) ([e5b4eff](https://github.com/openkcm/cmk/commit/e5b4effd5c94a0eeabf05b116c87a967f6414200))


### Bug Fixes

* add missing landscape config to event reconciler ([#242](https://github.com/openkcm/cmk/issues/242)) ([74803f5](https://github.com/openkcm/cmk/commit/74803f52af71d477a530768a414ef40bdbdc9ffe))
* **deps:** bump the gomod-group group across 1 directory with 10 updates ([#240](https://github.com/openkcm/cmk/issues/240)) ([598997b](https://github.com/openkcm/cmk/commit/598997b972ef13e2c403c93e7bbb361b1dab8615))
* event error too big ([#250](https://github.com/openkcm/cmk/issues/250)) ([53b2be9](https://github.com/openkcm/cmk/commit/53b2be9786525891af18aacf0d2c0474cd17e263))
* fix broken unit test ([#247](https://github.com/openkcm/cmk/issues/247)) ([7eabe88](https://github.com/openkcm/cmk/commit/7eabe882ac1990f08a8aef7d137757921b115b90))
* get latest key version ID from DB for system events ([#232](https://github.com/openkcm/cmk/issues/232)) ([ccd1035](https://github.com/openkcm/cmk/commit/ccd1035fa3ea38289864d6c2811e300afc1a4856))
* import postgres driver for tracing ([#246](https://github.com/openkcm/cmk/issues/246)) ([4de3508](https://github.com/openkcm/cmk/commit/4de350829b42378a6268007f5244ad6f191b9d84))
* provided task config does not override Enabled flag if not specified ([#235](https://github.com/openkcm/cmk/issues/235)) ([0b7f1ce](https://github.com/openkcm/cmk/commit/0b7f1ce8f273a36ad42556d150688e8bc6aa9a05))
* remove unused blueprints ([#237](https://github.com/openkcm/cmk/issues/237)) ([cea3bed](https://github.com/openkcm/cmk/commit/cea3bedd27069f8f1052044c830f79b7d205495d))
* skip worklow expiry if transition is not available ([#241](https://github.com/openkcm/cmk/issues/241)) ([40a7023](https://github.com/openkcm/cmk/commit/40a702302ac6769d79c894f5738846b68bcc92f0))
* system error metadata join with wrong field ([#253](https://github.com/openkcm/cmk/issues/253)) ([05d4462](https://github.com/openkcm/cmk/commit/05d4462f2b1c7f7c912b03624b75d91f41a64111))
* systems filter by region ([#236](https://github.com/openkcm/cmk/issues/236)) ([b554d3f](https://github.com/openkcm/cmk/commit/b554d3f92c1440f870ee3de7652cf756f4a536f7))
* update tracing wrapper for DB to use multitenancy postgres ([#249](https://github.com/openkcm/cmk/issues/249)) ([f6b7f58](https://github.com/openkcm/cmk/commit/f6b7f585a708816b2226b2ecd994c39583aef680))

## [0.7.0](https://github.com/openkcm/cmk/compare/v0.6.1...v0.7.0) (2026-04-09)


### Features

* add traces creation ([#203](https://github.com/openkcm/cmk/issues/203)) ([5b9abf7](https://github.com/openkcm/cmk/commit/5b9abf7b833e93eccf9a736c438bc44298b7499d))
* Configurable crypto certs ([#214](https://github.com/openkcm/cmk/issues/214)) ([1d8ecdf](https://github.com/openkcm/cmk/commit/1d8ecdff6bea6d8b34c99ebbe4a1e762cdb2f479))
* database auth properties ([#211](https://github.com/openkcm/cmk/issues/211)) ([38ba08a](https://github.com/openkcm/cmk/commit/38ba08a0213209f48e516a73a442be0c6121e3d9))
* refactor key rotate event ([#216](https://github.com/openkcm/cmk/issues/216)) ([f17aa7e](https://github.com/openkcm/cmk/commit/f17aa7e1a19c7aed890d24c6d0d91cc7e87ee2bc))


### Bug Fixes

* change keystore pool monitor to OTLP metric ([#220](https://github.com/openkcm/cmk/issues/220)) ([faa1596](https://github.com/openkcm/cmk/commit/faa1596d5ba33504c775ffe5c7d943b0af627e49))
* **deps:** bump the gomod-group group across 1 directory with 7 updates ([#213](https://github.com/openkcm/cmk/issues/213)) ([488db22](https://github.com/openkcm/cmk/commit/488db22e6c2abde480d7a4d174e22635fea1e80a))
* **deps:** bump the gomod-group group across 1 directory with 9 updates ([#225](https://github.com/openkcm/cmk/issues/225)) ([6f3072f](https://github.com/openkcm/cmk/commit/6f3072f48b85028c4333338b34d057c032df0ed0))
* refactor tenant decommissioning ([#175](https://github.com/openkcm/cmk/issues/175)) ([ba4f90d](https://github.com/openkcm/cmk/commit/ba4f90d7ba5c4f669dd53fdd2161e62130ab65fd))
* refresh repo authz data ([#230](https://github.com/openkcm/cmk/issues/230)) ([445ca41](https://github.com/openkcm/cmk/commit/445ca4127160f302cd176b3a367d3896c53eb468))
* tidy go mod ([#226](https://github.com/openkcm/cmk/issues/226)) ([abe9ae0](https://github.com/openkcm/cmk/commit/abe9ae096fb1d59f499f0dd47ef9d3400bb5212d))
* update common-sdk ([#218](https://github.com/openkcm/cmk/issues/218)) ([22cee0f](https://github.com/openkcm/cmk/commit/22cee0f3d7a60386b496009b30e76e12dbc47f24))
* Update Go version ([#215](https://github.com/openkcm/cmk/issues/215)) ([12d6a32](https://github.com/openkcm/cmk/commit/12d6a3226f62d691e27a2bf65eb5bf7761b0cffe))
* validate group roles when processing client data ([#227](https://github.com/openkcm/cmk/issues/227)) ([b0bcdd8](https://github.com/openkcm/cmk/commit/b0bcdd802218f32a67de77627588269c017325b2))

## [0.6.1](https://github.com/openkcm/cmk/compare/v0.6.0...v0.6.1) (2026-03-19)


### Bug Fixes

* dockerfile copy migrations files ([#209](https://github.com/openkcm/cmk/issues/209)) ([c85c6a0](https://github.com/openkcm/cmk/commit/c85c6a02faec5720a08fad65c97c4d096d375669))

## [0.6.0](https://github.com/openkcm/cmk/compare/v0.5.0...v0.6.0) (2026-03-18)


### Features

* return default keystore support regions ([#166](https://github.com/openkcm/cmk/issues/166)) ([5b3a8bb](https://github.com/openkcm/cmk/commit/5b3a8bbc5bdc05a9d997da04233249834485f953))


### Bug Fixes

* load and use keystore operations & management plugins via new interface ([#206](https://github.com/openkcm/cmk/issues/206)) ([0bf62e4](https://github.com/openkcm/cmk/commit/0bf62e4e565d842226737bcb0cc47f347858865f))
* Orbital Database Setup  ([#204](https://github.com/openkcm/cmk/issues/204)) ([ca4b642](https://github.com/openkcm/cmk/commit/ca4b6420ebed68d6bd241e8bb9e43a6b6ff73dc7))

## [0.5.0](https://github.com/openkcm/cmk/compare/v0.4.1...v0.5.0) (2026-03-16)


### Features

* change default tenant certificate subject ([#168](https://github.com/openkcm/cmk/issues/168)) ([6f7b689](https://github.com/openkcm/cmk/commit/6f7b68928d59ea7b09b39a9cb436b2ff89805f16))
* Forward auth client_id ([#199](https://github.com/openkcm/cmk/issues/199)) ([03eee70](https://github.com/openkcm/cmk/commit/03eee708782e1f6610b4e1e146533605897f19bd))


### Bug Fixes

* change cert issuer, IDM and notification plugins to use go interfaces ([#184](https://github.com/openkcm/cmk/issues/184)) ([6c27d4d](https://github.com/openkcm/cmk/commit/6c27d4d569b84bb185a6a62dc31e84ec4032c67f))
* **deps:** bump github.com/getkin/kin-openapi from 0.133.0 to 0.134.0 in the gomod-group group ([#198](https://github.com/openkcm/cmk/issues/198)) ([d80ac03](https://github.com/openkcm/cmk/commit/d80ac031f50f8e5e433ad7905074abe4c16f63b0))
* **deps:** bump the gomod-group group with 3 updates ([#194](https://github.com/openkcm/cmk/issues/194)) ([7fe3dac](https://github.com/openkcm/cmk/commit/7fe3dac22638583812fee3a838bb559fc75f7e55))
* Fix order when listing systems ([#196](https://github.com/openkcm/cmk/issues/196)) ([662270b](https://github.com/openkcm/cmk/commit/662270b6cafe4d01b950e6693952645429dfa6e9))
* group rename expand and db-migrator to goose provider ([#156](https://github.com/openkcm/cmk/issues/156)) ([0e11e6c](https://github.com/openkcm/cmk/commit/0e11e6c166a48025fc2aa3ced875875e92507281))
* sql migration files required for db-migrator ([#197](https://github.com/openkcm/cmk/issues/197)) ([4b1a52d](https://github.com/openkcm/cmk/commit/4b1a52d27a85ddaeae884d5736a013a4f106266e))
* update db-migrator command to support dynamic command configuration ([#192](https://github.com/openkcm/cmk/issues/192)) ([3368ec5](https://github.com/openkcm/cmk/commit/3368ec5b96a57b36a8bb34d250445f25777dba6e))
* Update dependabot config ([#193](https://github.com/openkcm/cmk/issues/193)) ([0272ae6](https://github.com/openkcm/cmk/commit/0272ae6b9fbebc70d5e43a3b044423c385797ddd))

## [0.4.1](https://github.com/openkcm/cmk/compare/v0.4.0...v0.4.1) (2026-03-06)


### Bug Fixes

* publish workflow updated including the image signing and composite image ([#176](https://github.com/openkcm/cmk/issues/176)) ([c995207](https://github.com/openkcm/cmk/commit/c995207929332f5bd92d394052103322df94f780))

## [0.4.0](https://github.com/openkcm/cmk/compare/v0.3.0...v0.4.0) (2026-03-06)


### Features

* add noop plugins ([#136](https://github.com/openkcm/cmk/issues/136)) ([3935230](https://github.com/openkcm/cmk/commit/3935230acf3e26ae616c128db99b10b73a3a9a5e))
* Add Tenant Name ([#110](https://github.com/openkcm/cmk/issues/110)) ([d9e548f](https://github.com/openkcm/cmk/commit/d9e548f84e33bdbc0bee7527663ae8f71de760bf))
* deploy data migrator post hook ([#87](https://github.com/openkcm/cmk/issues/87)) ([81d2149](https://github.com/openkcm/cmk/commit/81d2149d00cbf1cf806addcb9d4ce3097be8c28e))
* Enable workflow for primary key switch ([#126](https://github.com/openkcm/cmk/issues/126)) ([5191639](https://github.com/openkcm/cmk/commit/519163968691731f883bfaea48da3877f33cd7aa))
* grant key admin permission to access tenantInfo ([#155](https://github.com/openkcm/cmk/issues/155)) ([7e968fd](https://github.com/openkcm/cmk/commit/7e968fdd0fe7a1aae3292abd9d59ce69e93e4c75))
* include the SCIM identity management as builtin plugin ([#77](https://github.com/openkcm/cmk/issues/77)) ([939439e](https://github.com/openkcm/cmk/commit/939439ec314f278486176e972e4e64317b4a72ff))
* order systems by identifier ascending ([#149](https://github.com/openkcm/cmk/issues/149)) ([4c2d1ff](https://github.com/openkcm/cmk/commit/4c2d1fffea4a8b7409d04bd49f3346d3c8452d08))
* remove mixed roles check for allow list APIs ([#160](https://github.com/openkcm/cmk/issues/160)) ([3607c24](https://github.com/openkcm/cmk/commit/3607c242b249762e31c2463ed506dc13576453b2))


### Bug Fixes

* add dockerfiles to be used to create different images ([#170](https://github.com/openkcm/cmk/issues/170)) ([103f816](https://github.com/openkcm/cmk/commit/103f81644cf5ac5dea2e125e02ed0a6b964f559d))
* add missing sections in reconciler cfgmap ([#145](https://github.com/openkcm/cmk/issues/145)) ([bb69cb3](https://github.com/openkcm/cmk/commit/bb69cb3bef645bcfdda41b7c2c85d3b85fcb70d4))
* add plugin service api and wrappers from plugin-sdk ([#125](https://github.com/openkcm/cmk/issues/125)) ([7d8818e](https://github.com/openkcm/cmk/commit/7d8818e334c4f4293797d457a3796fd84f006fae))
* add plugins to event reconciler configmap ([#150](https://github.com/openkcm/cmk/issues/150)) ([878585a](https://github.com/openkcm/cmk/commit/878585a271e65f773ab5f4643a55cc58e3445c51))
* add sonar separate workflow ([#173](https://github.com/openkcm/cmk/issues/173)) ([eb15052](https://github.com/openkcm/cmk/commit/eb1505255bdb64b7c29c4b4803366800e763f907))
* change keyIDTo for system events on pkey change ([#131](https://github.com/openkcm/cmk/issues/131)) ([819af68](https://github.com/openkcm/cmk/commit/819af68a22c26dc4aa52df7327eaf5dd56809257))
* change tenant-manager podDisruptionBudgets name and labels ([#147](https://github.com/openkcm/cmk/issues/147)) ([90628bc](https://github.com/openkcm/cmk/commit/90628bce9e99ac2eb5f05964cd7a5277f84ee80f))
* include static configuration for identity management builtin plugin ([#157](https://github.com/openkcm/cmk/issues/157)) ([dfb83c6](https://github.com/openkcm/cmk/commit/dfb83c62771b0947306e13e6dbc7201049c6bb0a))
* remove usage name of plugins for single plugins ([#151](https://github.com/openkcm/cmk/issues/151)) ([247ac75](https://github.com/openkcm/cmk/commit/247ac7538f3367212f6a4ef2a280a4cf419ee524))
* update the plugin-sdk version introducing back buildinfo ([#146](https://github.com/openkcm/cmk/issues/146)) ([7acc30c](https://github.com/openkcm/cmk/commit/7acc30c73614a5eec9c75cf6cba2592d571786a0))
* use system user context for all batched periodic tasks ([#130](https://github.com/openkcm/cmk/issues/130)) ([9019edc](https://github.com/openkcm/cmk/commit/9019edc9bbf832dd649509a20f52d33f298050b1))
* wrong ctx on tasks ([#161](https://github.com/openkcm/cmk/issues/161)) ([8b63a07](https://github.com/openkcm/cmk/commit/8b63a07f68893b93a5a95423aa7c72c314b9b2c9))

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
