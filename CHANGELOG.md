# Changelog

## 20260714080036-restore-module-support

Restored support for `module` blocks in configuration files. You can now package a set of resources into a reusable module, instantiate it multiple times with different variable values, and nest modules inside other modules. Each module instance produces its own independent set of resources, resources inside a module can reference each other and the module's own variables, and a module instance can be disabled (which also disables all of its resources) while a resource inside a module can still independently control its own disabled state. Only local, relative-path module sources are supported; remote or URL-based module sources are not part of this change.
