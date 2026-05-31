cmake_minimum_required(VERSION 3.16)
project(runner-toywasm C)

# The build script passes TOYWASM_SRC / TOYWASM_BUILD; fall back to the
# in-tree cache layout. We consume toywasm via its `cmake --install` prefix
# (the only layout that exposes the <toywasm/...> headers the API uses).
if(NOT DEFINED TOYWASM_SRC)
    set(TOYWASM_SRC "${CMAKE_SOURCE_DIR}/../../.cache/runtimes/toywasm/src")
endif()
if(NOT DEFINED TOYWASM_BUILD)
    set(TOYWASM_BUILD "${TOYWASM_SRC}/../build")
endif()
if(NOT DEFINED TOYWASM_INSTALL)
    set(TOYWASM_INSTALL "${TOYWASM_SRC}/../install")
endif()

add_executable(runner-toywasm runner.c)
target_include_directories(runner-toywasm PRIVATE
    "${TOYWASM_INSTALL}/include"
)

find_package(Threads REQUIRED)
# libtoywasm-wasi depends on libtoywasm-core; list core last so the static
# linker resolves the back-references.
target_link_libraries(runner-toywasm
    "${TOYWASM_INSTALL}/lib/libtoywasm-wasi.a"
    "${TOYWASM_INSTALL}/lib/libtoywasm-core.a"
    Threads::Threads
    ${CMAKE_DL_LIBS}
    m
)
