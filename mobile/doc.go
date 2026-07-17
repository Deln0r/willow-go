// Package mobile is the gomobile-bindable surface of willow-go. It exposes a
// subset of the data-model and willow25 APIs through types gomobile can
// translate to Java (Android AAR) and Objective-C (iOS framework). The full Go
// API in datamodel, meadowcap, and willow25 is not bindable as-is: it uses
// [][]byte, *[]byte, custom interfaces, and pointer receivers in ways gomobile
// does not support directly.
//
// To build:
//
//	gomobile bind -target=ios     ./mobile   # produces Mobile.xcframework
//	gomobile bind -target=android ./mobile   # produces mobile.aar, all four ABIs
//
// Both bindings are built end-to-end; see README.md for the full workflow.
package mobile
