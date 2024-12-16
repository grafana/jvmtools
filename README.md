# jvmtools
A utility library for managing Hotspot JVM from Go

This package contains various tools for communicating with
the Hotspot JVM, i.e. OpenJDK.

The jattach part is a Go translation of https://github.com/jattach/jattach.
The component related to enabling DynamicAgentLoading is inspired by this 
repo https://github.com/xxDark/Unsolver-8306275.
