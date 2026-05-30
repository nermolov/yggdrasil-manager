cmake_minimum_required(VERSION 3.16)
project(runner-wasm3 C)

# Expects WASM3_SRC_DIR to point to the wasm3 source tree
if(NOT DEFINED WASM3_SRC_DIR)
    set(WASM3_SRC_DIR "${CMAKE_SOURCE_DIR}/../../.cache/runtimes/wasm3/src")
endif()

add_executable(runner-wasm3 runner.c)
target_include_directories(runner-wasm3 PRIVATE
    "${WASM3_SRC_DIR}/source"
)
target_link_libraries(runner-wasm3
    "${WASM3_SRC_DIR}/../build/source/libm3.a"
    m
)
