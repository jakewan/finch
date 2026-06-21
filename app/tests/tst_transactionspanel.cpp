// Qt Quick Test entry point. QUICK_TEST_MAIN scans QUICK_TEST_SOURCE_DIR (set in
// CMake) for tst_*.qml at runtime; the specs load the panel via a directory import,
// so no C++ type registration is needed here.
#include <QtQuickTest/quicktest.h>

QUICK_TEST_MAIN(transactionspanel)
