/* runner-wasmedge: WasmEdge host runner for the WASM-runtime memory benchmark. */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include "wasmedge/wasmedge.h"

static void die(const char *msg) {
    fprintf(stderr, "runner-wasmedge: %s\n", msg);
    exit(1);
}

int main(int argc, char *argv[]) {
    if (argc < 3) {
        fprintf(stderr, "usage: runner-wasmedge <module.wasm> <doc.am>\n");
        return 1;
    }

    /* Configure interpreter mode (no AOT) */
    WasmEdge_ConfigureContext *cfg = WasmEdge_ConfigureCreate();
    WasmEdge_ConfigureCompilerSetOptimizationLevel(cfg, WasmEdge_CompilerOptimizationLevel_O0);

    WasmEdge_VMContext *vm = WasmEdge_VMCreate(cfg, NULL);
    WasmEdge_ConfigureDelete(cfg);
    if (!vm) die("WasmEdge_VMCreate failed");

    /* Set up WASI with doc path as argv[1] */
    WasmEdge_ImportObjectContext *wasi_obj =
        WasmEdge_VMGetImportModuleContext(vm, WasmEdge_HostRegistration_Wasi);

    const char *wasi_argv[] = { argv[1], argv[2] };
    WasmEdge_ModuleInstanceInitWASI(wasi_obj, wasi_argv, 2, NULL, 0, NULL, 0);

    WasmEdge_String func_name = WasmEdge_StringCreateByCString("_start");
    WasmEdge_Result res = WasmEdge_VMRunWasmFromFile(vm, argv[1], func_name, NULL, 0, NULL, 0);
    WasmEdge_StringDelete(func_name);

    if (!WasmEdge_ResultOK(res)) {
        fprintf(stderr, "runner-wasmedge: %s\n", WasmEdge_ResultGetMessage(res));
        WasmEdge_VMDelete(vm);
        return 1;
    }

    WasmEdge_VMDelete(vm);
    return 0;
}
