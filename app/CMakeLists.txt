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
    QML_FILES qml/Main.qml qml/BalanceChart.qml qml/CreateAccountForm.qml
)

# --- Qt Quick Test ---------------------------------------------------------
# Kept in its own find_package so the production app build above does not depend
# on the test toolchain being installed.
find_package(Qt6 REQUIRED COMPONENTS QuickTest)
enable_testing()

add_executable(tst_createaccountform tests/tst_createaccountform.cpp)

# QUICK_TEST_MAIN scans this directory for tst_*.qml at runtime. The specs load
# the real form via a directory import ("../qml"), so the component under test is
# the exact file the app ships — not a stub.
target_compile_definitions(tst_createaccountform PRIVATE
    QUICK_TEST_SOURCE_DIR="${CMAKE_CURRENT_SOURCE_DIR}/tests")

target_link_libraries(tst_createaccountform PRIVATE
    Qt6::Quick
    Qt6::QuickControls2
    Qt6::QuickTest
    Qt6::Qml
)

add_test(NAME tst_createaccountform COMMAND tst_createaccountform)

# Run headless so the suite works in CI and over SSH (no display required).
set_tests_properties(tst_createaccountform PROPERTIES
    ENVIRONMENT "QT_QPA_PLATFORM=offscreen")
