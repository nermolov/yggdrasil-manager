cmake_minimum_required(VERSION 3.16)
project(runner-wamr C)

if(NOT DEFINED WAMR_SRC_DIR)
    set(WAMR_SRC_DIR "${CMAKE_SOURCE_DIR}/../../.cache/runtimes/wamr/src")
endif()
if(NOT DEFINED USE_FAST_INTERP)
    set(USE_FAST_INTERP 0)
endif()

if(USE_FAST_INTERP)
    set(BUILD_DIR "${WAMR_SRC_DIR}/../build-fast")
    set(TARGET_NAME "runner-wamr-fast")
else()
    set(BUILD_DIR "${WAMR_SRC_DIR}/../build-classic")
    set(TARGET_NAME "runner-wamr-classic")
endif()

add_executable(${TARGET_NAME} runner.c)
target_compile_definitions(${TARGET_NAME} PRIVATE USE_FAST_INTERP=${USE_FAST_INTERP})
target_include_directories(${TARGET_NAME} PRIVATE
    "${WAMR_SRC_DIR}/core/iwasm/include"
)
target_link_libraries(${TARGET_NAME}
    "${BUILD_DIR}/libiwasm.a"
    pthread dl m
)
