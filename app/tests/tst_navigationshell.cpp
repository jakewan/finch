// Qt Quick Test entry point. QUICK_TEST_MAIN scans QUICK_TEST_SOURCE_DIR (set in
// CMake to this target's own tests/navigationshell dir) for tst_*.qml at runtime; the
// specs load the shell via a directory import, so no C++ type registration is needed.
#include <QtQuickTest/quicktest.h>

QUICK_TEST_MAIN(navigationshell)
