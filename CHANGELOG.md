# Changelog

## [0.1.1](https://github.com/Omochice/nyctereutes/compare/v0.1.0...v0.1.1) (2026-07-08)


### Features

* add dep command to list, approve and merge dependency MRs ([#4](https://github.com/Omochice/nyctereutes/issues/4)) ([bdf26ab](https://github.com/Omochice/nyctereutes/commit/bdf26ab4279f4612881788b859ad35fb3b9cdc19))
* add group filter, browser open and refresh to the dep TUI ([#16](https://github.com/Omochice/nyctereutes/issues/16)) ([9303eed](https://github.com/Omochice/nyctereutes/commit/9303eed76030f7a416d359254f26b6ce03bceb4c))
* add infra plan subcommand showing manifest drift ([#30](https://github.com/Omochice/nyctereutes/issues/30)) ([7ab5d72](https://github.com/Omochice/nyctereutes/commit/7ab5d72ba195a61380cd9f08c0374904b8c11004))
* add infra validate subcommand for manifest YAML ([#29](https://github.com/Omochice/nyctereutes/issues/29)) ([8c26fc7](https://github.com/Omochice/nyctereutes/commit/8c26fc76586df366e32a62ee65f3255452cc0b4e))
* apply manifests to live GitLab state with infra apply ([#34](https://github.com/Omochice/nyctereutes/issues/34)) ([0b410cd](https://github.com/Omochice/nyctereutes/commit/0b410cdb97ec8576ace020a5aed6879400e73d97))
* classify glab API errors and act on them by class ([#35](https://github.com/Omochice/nyctereutes/issues/35)) ([41797be](https://github.com/Omochice/nyctereutes/commit/41797be0ffa230e90d5dfab623e15b436b823c6c))
* color the dep TUI CI status and unmergeable marker ([#15](https://github.com/Omochice/nyctereutes/issues/15)) ([45a4e30](https://github.com/Omochice/nyctereutes/commit/45a4e306f94d5563119f8a4ff676f3566d4dcfc7))
* emit multiline values as literal YAML blocks on infra import ([#26](https://github.com/Omochice/nyctereutes/issues/26)) ([02b6810](https://github.com/Omochice/nyctereutes/commit/02b6810dc7414e0fb1e2732d662950ba345f6488))
* export all GitLab project feature access levels on infra import ([#20](https://github.com/Omochice/nyctereutes/issues/20)) ([f0e7115](https://github.com/Omochice/nyctereutes/commit/f0e7115a35dba0b9b8b73f7be432c4302757ade4))
* export default_branch on infra import ([#28](https://github.com/Omochice/nyctereutes/issues/28)) ([3d987a6](https://github.com/Omochice/nyctereutes/commit/3d987a676022ed6b13a7a1fceaa98449decafe2e))
* export GitLab project feature access levels on infra import ([#19](https://github.com/Omochice/nyctereutes/issues/19)) ([80dad3f](https://github.com/Omochice/nyctereutes/commit/80dad3feda1116d3397ce01198d177098527e23b))
* export merge-related templates on infra import ([#24](https://github.com/Omochice/nyctereutes/issues/24)) ([f539c6c](https://github.com/Omochice/nyctereutes/commit/f539c6c8c2a50fa2794e6b1b4412aa1a6e162ced))
* export visibility-related boolean settings on infra import ([#21](https://github.com/Omochice/nyctereutes/issues/21)) ([f46edd9](https://github.com/Omochice/nyctereutes/commit/f46edd9d56b76331fe995a0f7029eedd761a8724))
* improve infra plan and apply diff output ([#37](https://github.com/Omochice/nyctereutes/issues/37)) ([e5c3cc2](https://github.com/Omochice/nyctereutes/commit/e5c3cc27dd2044a4fcd81fd3092fb29eb906f638))
* initialize ([8ce8d58](https://github.com/Omochice/nyctereutes/commit/8ce8d583a305ab73bb999a2086821d4ee0e5bb72))
* manage merge check settings in repository manifests ([#40](https://github.com/Omochice/nyctereutes/issues/40)) ([8295dc7](https://github.com/Omochice/nyctereutes/commit/8295dc736978d2c71b3feebacd0189e0c0be5163))
* manage merge_method in repository manifests ([#39](https://github.com/Omochice/nyctereutes/issues/39)) ([5aa4024](https://github.com/Omochice/nyctereutes/commit/5aa402468d8834e9908b185eefc564ede3ba860e))
* open an interactive TUI when "dep" is run with no subcommand ([#13](https://github.com/Omochice/nyctereutes/issues/13)) ([ae54481](https://github.com/Omochice/nyctereutes/commit/ae544812f4ec99213223995ddc3c607e12f56255))
* report build version via version command and --version flag ([#38](https://github.com/Omochice/nyctereutes/issues/38)) ([e965f2f](https://github.com/Omochice/nyctereutes/commit/e965f2f5a429d1a6a2c4c38bfc9b2fe78f9ccd69))
* report drift in all repository fields on infra plan ([#32](https://github.com/Omochice/nyctereutes/issues/32)) ([5866d68](https://github.com/Omochice/nyctereutes/commit/5866d68be5508a79b987c1df7ecb289397975800))
* scaffold nyctereutes CLI with subcommand dispatch ([#3](https://github.com/Omochice/nyctereutes/issues/3)) ([9b37b2b](https://github.com/Omochice/nyctereutes/commit/9b37b2ba369f9a41fdb7a1add8c403131e680328))
* toggle merge method and require-checks in the dep TUI ([#14](https://github.com/Omochice/nyctereutes/issues/14)) ([3163edc](https://github.com/Omochice/nyctereutes/commit/3163edc4e1daf838d9d7d61c62c4dec19fbb0939))


### Miscellaneous Chores

* exclude CHANGELOG.md from treefmt formatting ([#45](https://github.com/Omochice/nyctereutes/issues/45)) ([2d429ba](https://github.com/Omochice/nyctereutes/commit/2d429bac5e8668c8e3d19c07e06a0b6f2cbe5655))


### Continuous Integration

* give gh a repository context in the publish job ([#47](https://github.com/Omochice/nyctereutes/issues/47)) ([91b46ee](https://github.com/Omochice/nyctereutes/commit/91b46eee27684b0c2544351fa4f9b7b718e2b3b2))

## [0.1.0](https://github.com/Omochice/nyctereutes/compare/v0.1.0...v0.1.0) (2026-07-08)


### Features

* add dep command to list, approve and merge dependency MRs ([#4](https://github.com/Omochice/nyctereutes/issues/4)) ([bdf26ab](https://github.com/Omochice/nyctereutes/commit/bdf26ab4279f4612881788b859ad35fb3b9cdc19))
* add group filter, browser open and refresh to the dep TUI ([#16](https://github.com/Omochice/nyctereutes/issues/16)) ([9303eed](https://github.com/Omochice/nyctereutes/commit/9303eed76030f7a416d359254f26b6ce03bceb4c))
* add infra plan subcommand showing manifest drift ([#30](https://github.com/Omochice/nyctereutes/issues/30)) ([7ab5d72](https://github.com/Omochice/nyctereutes/commit/7ab5d72ba195a61380cd9f08c0374904b8c11004))
* add infra validate subcommand for manifest YAML ([#29](https://github.com/Omochice/nyctereutes/issues/29)) ([8c26fc7](https://github.com/Omochice/nyctereutes/commit/8c26fc76586df366e32a62ee65f3255452cc0b4e))
* apply manifests to live GitLab state with infra apply ([#34](https://github.com/Omochice/nyctereutes/issues/34)) ([0b410cd](https://github.com/Omochice/nyctereutes/commit/0b410cdb97ec8576ace020a5aed6879400e73d97))
* classify glab API errors and act on them by class ([#35](https://github.com/Omochice/nyctereutes/issues/35)) ([41797be](https://github.com/Omochice/nyctereutes/commit/41797be0ffa230e90d5dfab623e15b436b823c6c))
* color the dep TUI CI status and unmergeable marker ([#15](https://github.com/Omochice/nyctereutes/issues/15)) ([45a4e30](https://github.com/Omochice/nyctereutes/commit/45a4e306f94d5563119f8a4ff676f3566d4dcfc7))
* emit multiline values as literal YAML blocks on infra import ([#26](https://github.com/Omochice/nyctereutes/issues/26)) ([02b6810](https://github.com/Omochice/nyctereutes/commit/02b6810dc7414e0fb1e2732d662950ba345f6488))
* export all GitLab project feature access levels on infra import ([#20](https://github.com/Omochice/nyctereutes/issues/20)) ([f0e7115](https://github.com/Omochice/nyctereutes/commit/f0e7115a35dba0b9b8b73f7be432c4302757ade4))
* export default_branch on infra import ([#28](https://github.com/Omochice/nyctereutes/issues/28)) ([3d987a6](https://github.com/Omochice/nyctereutes/commit/3d987a676022ed6b13a7a1fceaa98449decafe2e))
* export GitLab project feature access levels on infra import ([#19](https://github.com/Omochice/nyctereutes/issues/19)) ([80dad3f](https://github.com/Omochice/nyctereutes/commit/80dad3feda1116d3397ce01198d177098527e23b))
* export merge-related templates on infra import ([#24](https://github.com/Omochice/nyctereutes/issues/24)) ([f539c6c](https://github.com/Omochice/nyctereutes/commit/f539c6c8c2a50fa2794e6b1b4412aa1a6e162ced))
* export visibility-related boolean settings on infra import ([#21](https://github.com/Omochice/nyctereutes/issues/21)) ([f46edd9](https://github.com/Omochice/nyctereutes/commit/f46edd9d56b76331fe995a0f7029eedd761a8724))
* improve infra plan and apply diff output ([#37](https://github.com/Omochice/nyctereutes/issues/37)) ([e5c3cc2](https://github.com/Omochice/nyctereutes/commit/e5c3cc27dd2044a4fcd81fd3092fb29eb906f638))
* initialize ([8ce8d58](https://github.com/Omochice/nyctereutes/commit/8ce8d583a305ab73bb999a2086821d4ee0e5bb72))
* manage merge check settings in repository manifests ([#40](https://github.com/Omochice/nyctereutes/issues/40)) ([8295dc7](https://github.com/Omochice/nyctereutes/commit/8295dc736978d2c71b3feebacd0189e0c0be5163))
* manage merge_method in repository manifests ([#39](https://github.com/Omochice/nyctereutes/issues/39)) ([5aa4024](https://github.com/Omochice/nyctereutes/commit/5aa402468d8834e9908b185eefc564ede3ba860e))
* open an interactive TUI when "dep" is run with no subcommand ([#13](https://github.com/Omochice/nyctereutes/issues/13)) ([ae54481](https://github.com/Omochice/nyctereutes/commit/ae544812f4ec99213223995ddc3c607e12f56255))
* report build version via version command and --version flag ([#38](https://github.com/Omochice/nyctereutes/issues/38)) ([e965f2f](https://github.com/Omochice/nyctereutes/commit/e965f2f5a429d1a6a2c4c38bfc9b2fe78f9ccd69))
* report drift in all repository fields on infra plan ([#32](https://github.com/Omochice/nyctereutes/issues/32)) ([5866d68](https://github.com/Omochice/nyctereutes/commit/5866d68be5508a79b987c1df7ecb289397975800))
* scaffold nyctereutes CLI with subcommand dispatch ([#3](https://github.com/Omochice/nyctereutes/issues/3)) ([9b37b2b](https://github.com/Omochice/nyctereutes/commit/9b37b2ba369f9a41fdb7a1add8c403131e680328))
* toggle merge method and require-checks in the dep TUI ([#14](https://github.com/Omochice/nyctereutes/issues/14)) ([3163edc](https://github.com/Omochice/nyctereutes/commit/3163edc4e1daf838d9d7d61c62c4dec19fbb0939))


### Miscellaneous Chores

* exclude CHANGELOG.md from treefmt formatting ([#45](https://github.com/Omochice/nyctereutes/issues/45)) ([2d429ba](https://github.com/Omochice/nyctereutes/commit/2d429bac5e8668c8e3d19c07e06a0b6f2cbe5655))
