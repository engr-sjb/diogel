package features

// FeatureLocation represents any feature name added to the features package.
// Whenever you create a new feature in the features package, you need to create
// a variable within that feature to holds the feature's name in that feature
// package.
//
// This mainly to help to identifying where errors are occurring faster in the
// features package.
//
// eg. var featureUser FeatureLocation = "user"
type FeatureLocation string
