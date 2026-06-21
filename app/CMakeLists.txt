cmake_minimum_required(VERSION 3.21)
project(finch-app VERSION 0.1.0 LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_AUTOMOC ON)

find_package(Qt6 REQUIRED COMPONENTS Quick QuickControls2 Concurrent Charts Widgets)
find_package(Protobuf REQUIRED)
find_package(gRPC CONFIG QUIET)

if(NOT gRPC_FOUND)
    find_package(PkgConfig REQUIRED)
    pkg_check_modules(GRPC REQUIRED grpc++ grpc)
endif()

# Generate protobuf and gRPC C++ code
set(PROTO_DIR "${CMAKE_SOURCE_DIR}/../proto")
set(PROTO_FILE "${PROTO_DIR}/finch/v1/finch.proto")
set(GENERATED_DIR "${CMAKE_BINARY_DIR}/gen")
file(MAKE_DIRECTORY ${GENERATED_DIR})

# Find grpc_cpp_plugin
find_program(GRPC_CPP_PLUGIN grpc_cpp_plugin)
if(NOT GRPC_CPP_PLUGIN)
    message(FATAL_ERROR "grpc_cpp_plugin not found")
endif()

# Generate protobuf sources
add_custom_command(
    OUTPUT
        "${GENERATED_DIR}/finch/v1/finch.pb.cc"
        "${GENERATED_DIR}/finch/v1/finch.pb.h"
        "${GENERATED_DIR}/finch/v1/finch.grpc.pb.cc"
        "${GENERATED_DIR}/finch/v1/finch.grpc.pb.h"
    COMMAND ${CMAKE_COMMAND} -E make_directory "${GENERATED_DIR}/finch/v1"
    COMMAND ${Protobuf_PROTOC_EXECUTABLE}
        "--proto_path=${PROTO_DIR}"
        "--cpp_out=${GENERATED_DIR}"
        "--grpc_out=${GENERATED_DIR}"
        "--plugin=protoc-gen-grpc=${GRPC_CPP_PLUGIN}"
        "${PROTO_FILE}"
    DEPENDS "${PROTO_FILE}"
    COMMENT "Generating C++ protobuf/gRPC code"
)

add_executable(finch-app
    src/main.cpp
    src/finchclient.h
    src/finchclient.cpp
    "${GENERATED_DIR}/finch/v1/finch.pb.cc"
    "${GENERATED_DIR}/finch/v1/finch.grpc.pb.cc"
)

target_include_directories(finch-app PRIVATE ${GENERATED_DIR})

target_link_libraries(finch-app PRIVATE
    Qt6::Quick
    Qt6::QuickControls2
    Qt6::Concurrent
    Qt6::Charts
    Qt6::Widgets
    protobuf::libprotobuf
)

if(gRPC_FOUND)
    target_link_libraries(finch-app PRIVATE gRPC::grpc++)
else()
    target_include_directories(finch-app PRIVATE ${GRPC_INCLUDE_DIRS})
    target_link_libraries(finch-app PRIVATE ${GRPC_LIBRARIES})
endif()

qt_add_qml_module(finch-app
    URI Finch
    VERSION 1.0
    QML_FILES
        qml/Main.qml
        qml/BalanceChart.qml
        qml/CreateAccountForm.qml
        qml/NavigationShell.qml
        qml/AccountsPanel.qml
        qml/TransactionsPanel.qml
        qml/PlaceholderScreen.qml
)

# --- Qt Quick Test ---------------------------------------------------------
# QuickTest has its own find_package so Qt6::QuickTest is linked only into the
# test target below, never into finch-app. Configuring the project does still
# require the test toolchain (the README lists the packages); the test target is
# always built — `just build-app` limits itself to the finch-app target.
find_package(Qt6 REQUIRED COMPONENTS QuickTest)
enable_testing()

add_executable(tst_createaccountform tests/tst_createaccountform.cpp)

# A test-only QML module holding just the real shipping form and the mock client.
# Scoping the module to these two self-contained files (rather than importing the whole
# qml/ directory) keeps the test's runtime module dependencies down to what the form
# itself imports — it does not drag in Main.qml/BalanceChart.qml and their transitive
# modules (QtCharts, QtCore, QtQml.WorkerScript). CreateAccountForm.qml is the
# exact file the app ships, so the component under test is real, not a stub.
qt_add_qml_module(tst_createaccountform
    URI FinchFormTest
    VERSION 1.0
    QML_FILES
        qml/CreateAccountForm.qml
        qml/AccountsPanel.qml
        tests/MockFinchClient.qml
)

# QUICK_TEST_MAIN scans QUICK_TEST_SOURCE_DIR for tst_*.qml at runtime. Each test
# executable points at its OWN subdirectory: the scan is recursive and a shared dir would
# make every executable try to load every spec, failing on the test-only module a spec
# imports but its executable does not link.
target_compile_definitions(tst_createaccountform PRIVATE
    QUICK_TEST_SOURCE_DIR="${CMAKE_CURRENT_SOURCE_DIR}/tests/createaccountform")

target_link_libraries(tst_createaccountform PRIVATE
    Qt6::Quick
    Qt6::QuickControls2
    Qt6::QuickTest
    Qt6::Qml
)

add_test(NAME tst_createaccountform COMMAND tst_createaccountform)

# Run headless (no display required) and pin the dependency-light Basic Controls style,
# so the test is deterministic across machines and CI needn't carry other styles' modules.
set_tests_properties(tst_createaccountform PROPERTIES
    ENVIRONMENT "QT_QPA_PLATFORM=offscreen;QT_QUICK_CONTROLS_STYLE=Basic")

# --- NavigationShell test ---
add_executable(tst_navigationshell tests/tst_navigationshell.cpp)

# A test-only module scoped to the real shell + the placeholder it composes — both
# chart-free and client-free, so the shell is driven headless with no QtCharts or daemon.
qt_add_qml_module(tst_navigationshell
    URI FinchShellTest
    VERSION 1.0
    QML_FILES
        qml/NavigationShell.qml
        qml/PlaceholderScreen.qml
)

target_compile_definitions(tst_navigationshell PRIVATE
    QUICK_TEST_SOURCE_DIR="${CMAKE_CURRENT_SOURCE_DIR}/tests/navigationshell")

target_link_libraries(tst_navigationshell PRIVATE
    Qt6::Quick
    Qt6::QuickControls2
    Qt6::QuickTest
    Qt6::Qml
)

add_test(NAME tst_navigationshell COMMAND tst_navigationshell)

set_tests_properties(tst_navigationshell PROPERTIES
    ENVIRONMENT "QT_QPA_PLATFORM=offscreen;QT_QUICK_CONTROLS_STYLE=Basic")

# --- TransactionsPanel test ---
add_executable(tst_transactionspanel tests/tst_transactionspanel.cpp)

# A test-only module scoped to the real master-detail panel + the shared mock client. The
# panel imports only QtQuick/Controls/Layouts (no QtCharts), so the default QGuiApplication
# entry point is fine — the QApplication requirement is a QtCharts-only constraint.
qt_add_qml_module(tst_transactionspanel
    URI FinchTransactionsTest
    VERSION 1.0
    QML_FILES
        qml/TransactionsPanel.qml
        tests/MockFinchClient.qml
)

target_compile_definitions(tst_transactionspanel PRIVATE
    QUICK_TEST_SOURCE_DIR="${CMAKE_CURRENT_SOURCE_DIR}/tests/transactionspanel")

target_link_libraries(tst_transactionspanel PRIVATE
    Qt6::Quick
    Qt6::QuickControls2
    Qt6::QuickTest
    Qt6::Qml
)

add_test(NAME tst_transactionspanel COMMAND tst_transactionspanel)

set_tests_properties(tst_transactionspanel PROPERTIES
    ENVIRONMENT "QT_QPA_PLATFORM=offscreen;QT_QUICK_CONTROLS_STYLE=Basic")
