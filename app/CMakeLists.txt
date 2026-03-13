cmake_minimum_required(VERSION 3.21)
project(finch-app VERSION 0.1.0 LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_AUTOMOC ON)

find_package(Qt6 REQUIRED COMPONENTS Quick QuickControls2)
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
    COMMAND protobuf::protoc
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
    QML_FILES qml/Main.qml
)
