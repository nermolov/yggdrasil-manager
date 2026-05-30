cmake_minimum_required(VERSION 3.16)
project(runner-wasmedge C)

if(NOT DEFINED WASMEDGE_BUILD_DIR)
    set(WASMEDGE_BUILD_DIR "${CMAKE_SOURCE_DIR}/../../.cache/runtimes/wasmedge/build")
endif()

add_executable(runner-wasmedge runner.c)
target_include_directories(runner-wasmedge PRIVATE
    "${WASMEDGE_BUILD_DIR}/include"
    "${WASMEDGE_BUILD_DIR}/../src/include"
)
target_link_libraries(runner-wasmedge
    "${WASMEDGE_BUILD_DIR}/lib/libwasmedge.so"
    pthread dl m
)
