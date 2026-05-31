cmake_minimum_required(VERSION 3.16)
project(runner-wasm3 C)

# The build script passes WASM3_SRC / WASM3_BUILD; fall back to the in-tree
# cache layout and the older WASM3_SRC_DIR name for direct invocations.
if(NOT DEFINED WASM3_SRC)
    if(DEFINED WASM3_SRC_DIR)
        set(WASM3_SRC "${WASM3_SRC_DIR}")
    else()
        set(WASM3_SRC "${CMAKE_SOURCE_DIR}/../../.cache/runtimes/wasm3/src")
    endif()
endif()
if(NOT DEFINED WASM3_BUILD)
    set(WASM3_BUILD "${WASM3_SRC}/../build")
endif()

add_executable(runner-wasm3 runner.c)
target_include_directories(runner-wasm3 PRIVATE
    "${WASM3_SRC}/source"
)

target_link_libraries(runner-wasm3
    "${WASM3_BUILD}/source/libm3.a"
)

# wasm3's default BUILD_WASI=uvwasi pulls libm3 references to uvwasi/libuv.
# Link them (plus their pthread/dl deps) when present in the build tree.
set(_uvwasi_a "${WASM3_BUILD}/_deps/uvwasi-build/libuvwasi_a.a")
set(_libuv_a "${WASM3_BUILD}/_deps/libuv-build/libuv_a.a")
if(EXISTS "${_uvwasi_a}" AND EXISTS "${_libuv_a}")
    find_package(Threads REQUIRED)
    target_link_libraries(runner-wasm3
        "${_uvwasi_a}"
        "${_libuv_a}"
        Threads::Threads
        ${CMAKE_DL_LIBS}
    )
endif()

target_link_libraries(runner-wasm3 m)
