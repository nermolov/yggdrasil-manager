cmake_minimum_required(VERSION 3.16)
project(runner-toywasm C)

if(NOT DEFINED TOYWASM_BUILD_DIR)
    set(TOYWASM_BUILD_DIR "${CMAKE_SOURCE_DIR}/../../.cache/runtimes/toywasm/build")
endif()

add_executable(runner-toywasm runner.c)
target_include_directories(runner-toywasm PRIVATE
    "${TOYWASM_BUILD_DIR}/../src/lib"
    "${TOYWASM_BUILD_DIR}/include"
)
target_link_libraries(runner-toywasm
    "${TOYWASM_BUILD_DIR}/lib/libtoywasm.a"
    m pthread
)
