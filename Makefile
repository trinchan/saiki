.PHONY: build
build: vendor
	hack/build.sh

.PHONY: test
test:
	hack/test.sh

.PHONY: check_format
check_format:
	hack/check_format.sh

.PHONY: install_static_analysis
install_static_analyzer:
	hack/install_static_analyzer.sh

.PHONY: check_static_analysis
check_static_analysis:
	hack/check_static_analysis.sh

.PHONY: vendor
vendor:
	go mod tidy
	go mod vendor

.PHONY: clean
clean:
	rm saiki

.PHONY: ci
ci: test install_static_analyzer check_static_analysis check_format check_license

.PHONY: license
license:
	hack/apply_license.sh

.PHONY: check_license
check_license:
	hack/check_license.sh

.PHONY: push_image
push_image:
	hack/push_image.sh