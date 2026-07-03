.PHONY: help test bench smoketest mobile-ios mobile-android mobile-clean

# Show the available targets.
help:
	@echo "willow-go make targets:"
	@echo "  test            run all Go unit tests"
	@echo "  bench           run microbenchmarks (datamodel + willow25)"
	@echo "  smoketest       run the cross-impl byte-compat acceptance gate"
	@echo "  mobile-ios      build the iOS XCFramework (requires Xcode)"
	@echo "  mobile-android  build the Android AAR (requires NDK + JDK)"
	@echo "  mobile-clean    remove generated mobile artifacts"
	@echo "See HACKING.md for fixture regeneration and fuzzing."

# Run all Go unit tests.
test:
	go test ./...

# Run all microbenchmarks (see BENCHMARKS.md).
bench:
	go test -bench=. -benchmem -run=^$$ ./datamodel/ ./willow25/

# Run the cross-impl byte-compat acceptance gate.
smoketest:
	go run ./cmd/willow-smoketest

# Build the iOS XCFramework. Requires Xcode and the gomobile toolchain
# (go install golang.org/x/mobile/cmd/gomobile@latest && gomobile init).
# Output: ./Mobile.xcframework (drag into an Xcode project to use).
mobile-ios:
	gomobile bind -target=ios -o Mobile.xcframework ./mobile

# Build the Android AAR for all four ABIs. Requires Android NDK
# (set ANDROID_NDK_HOME), a JDK on PATH (JDK 17+), and the gomobile toolchain.
# Output: ./mobile.aar (drop into an Android project's libs/).
mobile-android:
	gomobile bind -target=android -androidapi=21 -o mobile.aar ./mobile

# Remove generated mobile artifacts.
mobile-clean:
	rm -rf Mobile.xcframework mobile.aar mobile-sources.jar
