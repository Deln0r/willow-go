.PHONY: test smoketest mobile-ios mobile-android mobile-clean

# Run all Go unit tests.
test:
	go test ./...

# Run the cross-impl byte-compat acceptance gate.
smoketest:
	go run ./cmd/willow-smoketest

# Build the iOS XCFramework. Requires Xcode and the gomobile toolchain
# (go install golang.org/x/mobile/cmd/gomobile@latest && gomobile init).
# Output: ./Mobile.xcframework (drag into an Xcode project to use).
mobile-ios:
	gomobile bind -target=ios -o Mobile.xcframework ./mobile

# Build the Android AAR. Requires Android NDK (set ANDROID_NDK_HOME),
# a JDK on PATH (Android SDK accepts JDK 17+), and the gomobile toolchain.
# Output: ./mobile.aar (drop into an Android project's libs/).
mobile-android:
	gomobile bind -target=android/arm64 -androidapi=21 -o mobile.aar ./mobile

# Remove generated mobile artifacts.
mobile-clean:
	rm -rf Mobile.xcframework mobile.aar mobile-sources.jar
