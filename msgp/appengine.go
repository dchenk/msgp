// +build appengine

package msgp

// Because we can't use package unsafe on AppEngine, let's assume the apps run on 64-bit hardware.
const smallint = false
