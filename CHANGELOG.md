# Changelog

## [0.9.0](https://github.com/openkcm/cmk/compare/v0.8.0...v0.9.0) (2026-08-18)


### Features

* add Content-Security-Policy header to API responses ([948934e](https://github.com/openkcm/cmk/commit/948934ea4a11b32466bc1cc99c67313231bc4253))
* add keystore pool percentage ([b7b7d3f](https://github.com/openkcm/cmk/commit/b7b7d3f47165c5bdfe8f7e5ed5d96e12911af49d))
* add support for db enum check ([52f48ae](https://github.com/openkcm/cmk/commit/52f48ae5b972327e89c4721c328107526355b8ab))
* Add target KeyConfig reference on System to expose in-flight Connect/Switch attempts ([#317](https://github.com/openkcm/cmk/issues/317)) ([4025c72](https://github.com/openkcm/cmk/commit/4025c72cdaea5a1a5785a018c0cdf76442307f88))
* add tenant termination timeout ([71162ae](https://github.com/openkcm/cmk/commit/71162ae5f45507b2dda571bba7a88fb3427158e3))
* add workflow settings validation ([1c778f7](https://github.com/openkcm/cmk/commit/1c778f7bd7ef386c45ec8ce4a74facd93f00f785))
* allow adding more regions on key crypto access details ([4e45bb5](https://github.com/openkcm/cmk/commit/4e45bb5aab4aed69a1af0e97fa78db14408a4f4c))
* better description and payload for hyok and byok ([#321](https://github.com/openkcm/cmk/issues/321)) ([f9c18f5](https://github.com/openkcm/cmk/commit/f9c18f57d8f6c7e417dd4938b125e6db80d72bb5))
* centralise system connect to keyconfig key logic ([#379](https://github.com/openkcm/cmk/issues/379)) ([3f1c715](https://github.com/openkcm/cmk/commit/3f1c71599af819ae7f8c169b8fd2bcd8ca72ab15))
* certificate subject struct ([78e5e85](https://github.com/openkcm/cmk/commit/78e5e857ea5200fa2dea4d3a6a61bf3fc96e0388))
* check for keys on keyconfig delete ([4c81132](https://github.com/openkcm/cmk/commit/4c8113297289287756a65d8dd8a9353334c3b79a))
* default iam name on missing entry ([#326](https://github.com/openkcm/cmk/issues/326)) ([6c06c89](https://github.com/openkcm/cmk/commit/6c06c89b740484c958475b31fb0d6874141bbd00))
* enable OTLP traces and logs ([#337](https://github.com/openkcm/cmk/issues/337)) ([9f84f45](https://github.com/openkcm/cmk/commit/9f84f45943296c0206a27257c908db0fac06f1e1))
* filter get system by keyconfig name ([b2ca9ae](https://github.com/openkcm/cmk/commit/b2ca9ae9b2f1afe649b38d32a9a31a2028f7dd56))
* handle AUTH_ACTION_REMOVE_AUTH task in tenant manager ([#340](https://github.com/openkcm/cmk/issues/340)) ([62cf6b9](https://github.com/openkcm/cmk/commit/62cf6b9c8d1d135bbc3b4d907e72fa0873515db5))
* handle certificate trust for BYOK ([dd56f94](https://github.com/openkcm/cmk/commit/dd56f942e577e60fc7216678d5a0adf3ec6d5e75))
* key state change and disable workflow, and remove under_workflow state ([cd5da81](https://github.com/openkcm/cmk/commit/cd5da818a944c9bb25eae323c21030dc972b9daa))
* local cache iam user ([#328](https://github.com/openkcm/cmk/issues/328)) ([5d32bbb](https://github.com/openkcm/cmk/commit/5d32bbbd369786ae1faf2269d39aa52ac4483213))
* read supportedRegions from CMK keystore pool config ([#363](https://github.com/openkcm/cmk/issues/363)) ([7073244](https://github.com/openkcm/cmk/commit/707324436c04a34bb1aed191e2e1bbacc2cee71a))
* remove cmkAvailable and cmkUnavailable ([6103f16](https://github.com/openkcm/cmk/commit/6103f163df27a58d513f15780790c7c664b5abe6))
* remove pkcs7 package ([04b472f](https://github.com/openkcm/cmk/commit/04b472f1f53ec0231deacf11530f928029240412))
* remove slog2hclog dependency ([71e6eab](https://github.com/openkcm/cmk/commit/71e6eab11e4cc4f7ae58ed5c3f387e98b1b3798d))
* replace gogo/protobuf with official google ([5c28629](https://github.com/openkcm/cmk/commit/5c286292bc91d4666635a101674fcccc02371246))
* return only active tenants on list tenants ([6534f37](https://github.com/openkcm/cmk/commit/6534f37cd19ae4d3985da9d1223c8bb17a42e3e6))
* set system status to LOCKED in registry on decommission failures ([8a9ffc1](https://github.com/openkcm/cmk/commit/8a9ffc16d3aa03f3241f3333e83beac126990bde))
* sort available keystore providers by name ([a9358c6](https://github.com/openkcm/cmk/commit/a9358c631e640e32c026f1221316039683265f85))
* update keystore role ([#318](https://github.com/openkcm/cmk/issues/318)) ([a3dc5de](https://github.com/openkcm/cmk/commit/a3dc5dee5774830e8dbade2e81d4988bc0dbdb77))
* update tenant by id retrieval ([#257](https://github.com/openkcm/cmk/issues/257)) ([1555224](https://github.com/openkcm/cmk/commit/1555224813555cc25365458df71379f491745c9c))
* validate workflow artifact and action type set ([ca9c1b1](https://github.com/openkcm/cmk/commit/ca9c1b119ca6d40a36fc3136d9ff0e0e4702c8ef))


### Bug Fixes

* add authz check for byok ([6727339](https://github.com/openkcm/cmk/commit/6727339e9017b4930064348e557158dcf064811d))
* add keystorePool supported regions configuration ([#336](https://github.com/openkcm/cmk/issues/336)) ([c3f7985](https://github.com/openkcm/cmk/commit/c3f79855af3eac82c1d33c1868c1a7609f5ac0b6))
* add missing internal permissions for key sync and key event ([11ee6e4](https://github.com/openkcm/cmk/commit/11ee6e482bf0cc09aadafa56714f4b30e9c6382c))
* add missing key version read permission for event-reconciler ([b87a6ce](https://github.com/openkcm/cmk/commit/b87a6ce930952ce6f3ecbda8984618ac38694cc6))
* Add missing permissions for InternalTenantProvisioningRole ([844b469](https://github.com/openkcm/cmk/commit/844b4696406802cc386c0e3935475aea1ee56daa))
* add missing permissions for tenant offboarding and workflow cleanup ([2773c20](https://github.com/openkcm/cmk/commit/2773c20c359f2716f92d0cb413e3e30d7143499b))
* add plugin watcher to detect dead plugin processes ([425295b](https://github.com/openkcm/cmk/commit/425295b6e50429c944690990f7f0610ff7cbf6a0))
* allow tenant provisioning role to update systems during decommis… ([#360](https://github.com/openkcm/cmk/issues/360)) ([7e63bf9](https://github.com/openkcm/cmk/commit/7e63bf9c29d3b55e739ea53ae0ebcdeb02f78fbd))
* byok import params ([#357](https://github.com/openkcm/cmk/issues/357)) ([d000256](https://github.com/openkcm/cmk/commit/d000256ae607451ec41dc869a18f05ac25dd6d2d))
* bypass repo authorization for get tenants ([1281e59](https://github.com/openkcm/cmk/commit/1281e59b8effaa9db02f49f4bf8be01581e8298b))
* change 403 to 400 on some errors ([3683643](https://github.com/openkcm/cmk/commit/36836430d2facb118e69b6f48e4b665675b5b294))
* configmap ([cddefb5](https://github.com/openkcm/cmk/commit/cddefb513a9f0e097edf38038f0d0bd8edde54a8))
* convert crypto access data before sending to plugin ([#369](https://github.com/openkcm/cmk/issues/369)) ([ce08a95](https://github.com/openkcm/cmk/commit/ce08a9560ced76bded1ce4776aa61ca508a378a1))
* correct function name casing for dropSchema in delete tenant command ([8618044](https://github.com/openkcm/cmk/commit/8618044c62bfa4628e679cacfac85ca383fac3c3))
* create dedicated check for workflow CanExpire ([#255](https://github.com/openkcm/cmk/issues/255)) ([e1fc067](https://github.com/openkcm/cmk/commit/e1fc0676ea09ea062bad238e6ff0cce3941b9d67))
* creator name empty ([aafad64](https://github.com/openkcm/cmk/commit/aafad64e5a272d3a5c783441d4364ba96e24f5fd))
* data migration missing schema checks ([9dc6104](https://github.com/openkcm/cmk/commit/9dc6104824df6d9ed57baf2c8874429caebd7b1b))
* db parameters on wrong place ([#268](https://github.com/openkcm/cmk/issues/268)) ([b913377](https://github.com/openkcm/cmk/commit/b913377915b8c593b267ae2d96765791db5636a5))
* **deps:** bump alpine from 3.23.3 to 3.23.4 in the docker-group group ([#251](https://github.com/openkcm/cmk/issues/251)) ([696c021](https://github.com/openkcm/cmk/commit/696c021c2f8d37a589a9be86adf7b4707fb8dfcf))
* **deps:** bump alpine from 3.23.4 to 3.24.1 in the docker-group group ([baa5ee4](https://github.com/openkcm/cmk/commit/baa5ee4168e76a99ef917444d9688b7469f662b0))
* **deps:** bump crypto version ([e2c7257](https://github.com/openkcm/cmk/commit/e2c72579bb69fb8d7cfcc69b03a78b254196cc0b))
* **deps:** bump distroless/static-debian12 from `a932952` to `b7bb25d` in the docker-group group ([#324](https://github.com/openkcm/cmk/issues/324)) ([492a081](https://github.com/openkcm/cmk/commit/492a081c6edb7e95ec037be461c6cbf26a833372))
* **deps:** bump distroless/static-debian12 from `aef9602` to `f5b485e` in the docker-group group across 1 directory ([#338](https://github.com/openkcm/cmk/issues/338)) ([56df8ae](https://github.com/openkcm/cmk/commit/56df8aea6ff2b81a55cfcefdce615f93d15920d8))
* **deps:** bump distroless/static-debian12 from `b7bb25d` to `aef9602` in the docker-group group ([#329](https://github.com/openkcm/cmk/issues/329)) ([36d9892](https://github.com/openkcm/cmk/commit/36d989222abac48af35760d42d223df92074d918))
* **deps:** bump distroless/static-debian12 from `f5b485e` to `1b7b9f0` in the docker-group group ([#373](https://github.com/openkcm/cmk/issues/373)) ([7731667](https://github.com/openkcm/cmk/commit/7731667e6ec11571b8619e5e20487b541efb7cc6))
* **deps:** bump github.com/getkin/kin-openapi from 0.143.0 to 0.144.0 in the gomod-group group ([#352](https://github.com/openkcm/cmk/issues/352)) ([9fb36cc](https://github.com/openkcm/cmk/commit/9fb36cc3317fc1d205de1fd1a3d700098c85b688))
* **deps:** bump github.com/getkin/kin-openapi from 0.144.0 to 0.145.0 in the gomod-group group ([#354](https://github.com/openkcm/cmk/issues/354)) ([5b2d00f](https://github.com/openkcm/cmk/commit/5b2d00f8c75d6a6e6741a43ec9ef21cc9f5a8742))
* **deps:** bump github.com/google/cel-go from 0.28.0 to 0.29.0 ([#353](https://github.com/openkcm/cmk/issues/353)) ([aec96f5](https://github.com/openkcm/cmk/commit/aec96f5ef21f2b8ad8c86065145c7b3e5afc8893))
* **deps:** bump github.com/jackc/pgx/v5 from 5.9.1 to 5.9.2 ([#264](https://github.com/openkcm/cmk/issues/264)) ([fbd85d2](https://github.com/openkcm/cmk/commit/fbd85d24556d19ecbaea936b1a8a14e3e9b51589))
* **deps:** bump github.com/pressly/goose/v3 from 3.27.2 to 3.27.3 in the gomod-group group across 1 directory ([#343](https://github.com/openkcm/cmk/issues/343)) ([d757559](https://github.com/openkcm/cmk/commit/d75755989a1e5a25b5eadd34336dfa56759d8894))
* **deps:** bump github.com/stretchr/testify from 1.11.1 to 1.12.0 in the gomod-group group ([#381](https://github.com/openkcm/cmk/issues/381)) ([fc2d44e](https://github.com/openkcm/cmk/commit/fc2d44e159c21d168b479a6696900dda6459d867))
* **deps:** bump google.golang.org/grpc from 1.82.0 to 1.82.1 in the gomod-group group ([#333](https://github.com/openkcm/cmk/issues/333)) ([595356f](https://github.com/openkcm/cmk/commit/595356fb5de10c1f8376cfd177e92bc515ef370a))
* **deps:** bump the gomod-group group across 1 directory with 18 updates ([#378](https://github.com/openkcm/cmk/issues/378)) ([574d491](https://github.com/openkcm/cmk/commit/574d491e98e8d940eb13441b7d9dc27ac39b27e2))
* **deps:** bump the gomod-group group across 1 directory with 4 updates ([#273](https://github.com/openkcm/cmk/issues/273)) ([bb125ed](https://github.com/openkcm/cmk/commit/bb125edf0043102bfe267f4728e54f1b5ae52ca8))
* **deps:** bump the gomod-group group across 1 directory with 4 updates ([#341](https://github.com/openkcm/cmk/issues/341)) ([360e3bc](https://github.com/openkcm/cmk/commit/360e3bc61ecc6ee4a73164a2d0dbeba69573ba77))
* **deps:** bump the gomod-group group across 1 directory with 5 updates ([#325](https://github.com/openkcm/cmk/issues/325)) ([a0c871f](https://github.com/openkcm/cmk/commit/a0c871fd384ebc99644c5ca6266e4007fd77bbdb))
* **deps:** bump the gomod-group group with 3 updates ([00388f7](https://github.com/openkcm/cmk/commit/00388f70028e20a41bd01eb0ad418a18251aacb5))
* **deps:** pin dependencies ([6f3e565](https://github.com/openkcm/cmk/commit/6f3e56529ab511c48956897055b17946c31e4148))
* **deps:** update all dependencies ([603bb2d](https://github.com/openkcm/cmk/commit/603bb2d28548f3cacc382ba96d24f6630b3f6e90))
* **deps:** update all dependencies ([0d3b5ce](https://github.com/openkcm/cmk/commit/0d3b5ce639994281ab61581cdf2b495604047994))
* **deps:** update all dependencies ([462b7b2](https://github.com/openkcm/cmk/commit/462b7b217944ef738054276c5c11a8fe1bdd16b9))
* **deps:** update all dependencies ([50d10db](https://github.com/openkcm/cmk/commit/50d10db50c562e8ef20abb276af15976e3d7238c))
* **deps:** update all dependencies ([fbc1319](https://github.com/openkcm/cmk/commit/fbc13193d438c1b2b833205851539fe12fceb873))
* **deps:** update all dependencies ([4402559](https://github.com/openkcm/cmk/commit/44025591d841082c5b786b78d180107d135f511d))
* **deps:** update all dependencies ([23e18a7](https://github.com/openkcm/cmk/commit/23e18a783cd9865d05987fc5455957aef6230cb3))
* **deps:** update all dependencies ([0c07449](https://github.com/openkcm/cmk/commit/0c074495fe237360707aaef657814cf7dfbd2178))
* disable auth and clientdata check on swagger endpoint ([8638825](https://github.com/openkcm/cmk/commit/8638825616052edabafb37cbcc8e6cf46483339e))
* disallow pkey switch with disabled pkey ([e14aa17](https://github.com/openkcm/cmk/commit/e14aa17120bd7cf19522941a027598fb6b6b4be3))
* do not override non sent values on region crypto ([#342](https://github.com/openkcm/cmk/issues/342)) ([110b2c7](https://github.com/openkcm/cmk/commit/110b2c76d4ba7b533c456ca66e35043094f2c022))
* dont get duplicateds on filter options ([af66025](https://github.com/openkcm/cmk/commit/af6602542ce2cc225bace5343b2f78edeceb5027))
* Fix BYOK enable/disable ([9f5ad5b](https://github.com/openkcm/cmk/commit/9f5ad5b89cfb2649d6cfc0422b3a247f2e855180))
* generic get key error on keystore ([79c5ed5](https://github.com/openkcm/cmk/commit/79c5ed59e0754ce12652b58be08168a3512adf1d))
* handle when supported regions source ref not configured ([7dd972d](https://github.com/openkcm/cmk/commit/7dd972d2745eafc503274bb77585417c87553dee))
* **helm:** make image.tag or image.digest mandatory ([e153bf2](https://github.com/openkcm/cmk/commit/e153bf2e26fcf57e60090e512024bb5fb6cf8efc))
* improve pattern extraction to prevent bypass attacks1 ([f5d84c3](https://github.com/openkcm/cmk/commit/f5d84c3a5cf949bee3e06b30508bd02e257c845f))
* is part of workflow groups check ([b6fbcc2](https://github.com/openkcm/cmk/commit/b6fbcc2e78d79aaf39f1edc19d357a6f21531a40))
* iseditable true by default, false on primary with connect system ([0e87f38](https://github.com/openkcm/cmk/commit/0e87f38b105c967e6f61693c2bc1973308f4a2d7))
* make workflow expiry task use separate transition function ([80673c0](https://github.com/openkcm/cmk/commit/80673c03c8362e99017ee2c81db272dbccc5c74b))
* missing action type on api ([2691154](https://github.com/openkcm/cmk/commit/2691154b931213267cc622f67af6941c0ee9782f))
* missing byok parameters when wrapping kms ([c1252af](https://github.com/openkcm/cmk/commit/c1252af2884595734e05e4045b8c7068e8f78198))
* missing filter on swagger example ([5732bbc](https://github.com/openkcm/cmk/commit/5732bbc7e926c7b1cb340b25534d8741ee159c4a))
* missing to update key underworkflow to false on termination ([00dfb5c](https://github.com/openkcm/cmk/commit/00dfb5c90c4c5655a580c696c1d484a29383f271))
* pin rabbitmq to 4.2 ([#267](https://github.com/openkcm/cmk/issues/267)) ([35ffe54](https://github.com/openkcm/cmk/commit/35ffe548b2d67654feeee663bb57bd33e439cb38))
* pkey switch must check for enabled key at source and target ([#330](https://github.com/openkcm/cmk/issues/330)) ([48fb002](https://github.com/openkcm/cmk/commit/48fb002aac1190e1aa9e3d7a5d968739ece8e669))
* pre-prepaired queries on otel db wrapper ([#263](https://github.com/openkcm/cmk/issues/263)) ([660003d](https://github.com/openkcm/cmk/commit/660003dcffbf4ca323c6b7c2c5d92fda18b93eb1))
* prevent early closure of TCP listener in GRPCSuite ([#366](https://github.com/openkcm/cmk/issues/366)) ([bb8c7b1](https://github.com/openkcm/cmk/commit/bb8c7b14f317f102d0e54249b44292203b498736))
* reconciliation loop ([#332](https://github.com/openkcm/cmk/issues/332)) ([d437ef6](https://github.com/openkcm/cmk/commit/d437ef6f4a1dbea08a2222f0854753d46f63b8d7))
* remove RSA key algorithms from supported types ([584c3c7](https://github.com/openkcm/cmk/commit/584c3c7de2c23316342be0da55fb565384ed7257))
* remove under workflow whenever workflow terminates ([66488f7](https://github.com/openkcm/cmk/commit/66488f7707bfabdebc12e631ace6cccc7d211e17))
* remove userid from db ([#223](https://github.com/openkcm/cmk/issues/223)) ([c6c8bb2](https://github.com/openkcm/cmk/commit/c6c8bb229ea320f2c1b54120c0c4801501d43c85))
* remove YAML config fallback on supported regions ([#376](https://github.com/openkcm/cmk/issues/376)) ([a84498a](https://github.com/openkcm/cmk/commit/a84498af66c0ed4f0c0a3f851d2ad710bcace2b3))
* replace github.com/docker/docker with github.com/moby/moby/api ([87202e9](https://github.com/openkcm/cmk/commit/87202e91cad93d27326a79db46d3a4e05e719725))
* reset system status to CONNECTED after SYSTEM_SWITCH job completes ([57e6ad9](https://github.com/openkcm/cmk/commit/57e6ad90433b0070c3f3e0b93368369391c5a92e))
* resolve fragile tests ([#334](https://github.com/openkcm/cmk/issues/334)) ([94e81a1](https://github.com/openkcm/cmk/commit/94e81a1dbfd22b680d5115159d12e8a50d0c0caf))
* return invalid state error when registering HYOK ([838b3b8](https://github.com/openkcm/cmk/commit/838b3b8ecd9a6cd314aeaa3aca0ea8547ba18adf))
* return INVALID_ACCESS_DATA on key update ([57519c7](https://github.com/openkcm/cmk/commit/57519c77988a166f9fb38e65a5c4c4266c0e4a4e))
* return nil creator info if creator id invalid ([ca03057](https://github.com/openkcm/cmk/commit/ca03057e906d925057b88c5f00b2ddfe3a63bb2b))
* return supportedRegions before keystore is retrieved from pool ([0707681](https://github.com/openkcm/cmk/commit/07076810e8055fc2a2ad8df3992b59265d0e47b0))
* revert HYOK default cert to use backward compatible purpose ([d31c32f](https://github.com/openkcm/cmk/commit/d31c32f51d3b3925fb3ad4b89fa242def23fac33))
* sanitise map[string]any ([b12cfb5](https://github.com/openkcm/cmk/commit/b12cfb592aaf918f1d604f1e769da254f7a2941d))
* **SEC-390:** add security response headers via common-sdk middleware ([b498bae](https://github.com/openkcm/cmk/commit/b498baeea5d8a08d36be2bb8d126029d402aefeb))
* set all false instead of error on non existing system event ([13db39c](https://github.com/openkcm/cmk/commit/13db39c9bc425719defb09b7e1881439610a410f))
* set system under workflow to false on workflow termination ([cf2bc25](https://github.com/openkcm/cmk/commit/cf2bc2567b6e986b72a90a340533ab64f276f6f1))
* shorted workflow async test case names ([fa77d01](https://github.com/openkcm/cmk/commit/fa77d018e4c508d8fc9428adb5effb9d061ea80e))
* test performance and local failures ([2f044a0](https://github.com/openkcm/cmk/commit/2f044a0b12a306817e3f5c037568b966d71fdef7))
* user client data issuer for get tenants ([df28d93](https://github.com/openkcm/cmk/commit/df28d935421a99c97df1977691f4835d11ef3f1f))
* values-dev no tls config ([#254](https://github.com/openkcm/cmk/issues/254)) ([770a829](https://github.com/openkcm/cmk/commit/770a8291de05373d1bae5578b8df2cbd63be51a8))
* workflow access for removed user ([8dc2cf3](https://github.com/openkcm/cmk/commit/8dc2cf3f1face1e96553964a5e8fc99dfb76a0f8))
* wrappers from api sdk ([#221](https://github.com/openkcm/cmk/issues/221)) ([eb7a8b9](https://github.com/openkcm/cmk/commit/eb7a8b9d2e45078704805c62b55704990d1a30ef))

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
