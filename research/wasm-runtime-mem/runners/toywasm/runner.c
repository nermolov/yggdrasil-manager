/* runner-toywasm: toywasm host runner for the WASM-runtime memory benchmark.
 *
 * Models toywasm's own examples/runwasi embedding: load the module, create a
 * WASI instance wired to the host stdio fds, instantiate, and run _start.
 * The doc payload is fed on fd 0 by the harness/dup2; the framed response is
 * written to fd 1. */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <unistd.h>

#include "toywasm/cconv.h"
#include "toywasm/exec_context.h"
#include "toywasm/instance.h"
#include "toywasm/load_context.h"
#include "toywasm/mem.h"
#include "toywasm/module.h"
#include "toywasm/report.h"
#include "toywasm/type.h"
#include "toywasm/wasi.h"

static void die(const char *msg) {
    fprintf(stderr, "runner-toywasm: %s\n", msg);
    exit(1);
}

static uint8_t *read_file(const char *path, size_t *out_len) {
    FILE *f = fopen(path, "rb");
    if (!f) { perror(path); return NULL; }
    fseek(f, 0, SEEK_END);
    long sz = ftell(f);
    rewind(f);
    uint8_t *buf = malloc(sz);
    if (!buf) { fclose(f); return NULL; }
    if (fread(buf, 1, sz, f) != (size_t)sz) { fclose(f); free(buf); return NULL; }
    fclose(f);
    *out_len = (size_t)sz;
    return buf;
}

int main(int argc, char *argv[]) {
    if (argc < 3) {
        fprintf(stderr, "usage: runner-toywasm <module.wasm> <doc.am>\n");
        return 1;
    }

    /* Stage the length-prefixed payload and dup2 it onto fd 0 so the WASI
     * fd 0 (inherited host stdin) reads it. */
    size_t doc_len = 0;
    uint8_t *doc_bytes = read_file(argv[2], &doc_len);
    if (!doc_bytes) die("failed to read doc");
    size_t stdin_len = 4 + doc_len;
    uint8_t *stdin_buf = malloc(stdin_len);
    if (!stdin_buf) die("malloc");
    stdin_buf[0] = (doc_len >> 24) & 0xff;
    stdin_buf[1] = (doc_len >> 16) & 0xff;
    stdin_buf[2] = (doc_len >>  8) & 0xff;
    stdin_buf[3] = (doc_len      ) & 0xff;
    memcpy(stdin_buf + 4, doc_bytes, doc_len);
    free(doc_bytes);
    FILE *tmp = tmpfile();
    if (!tmp) die("tmpfile");
    fwrite(stdin_buf, 1, stdin_len, tmp);
    fflush(tmp);
    rewind(tmp);
    free(stdin_buf);
    if (dup2(fileno(tmp), STDIN_FILENO) < 0) die("dup2 stdin");

    size_t wasm_len = 0;
    uint8_t *wasm_bytes = read_file(argv[1], &wasm_len);
    if (!wasm_bytes) die("failed to read wasm module");

    struct mem_context mctx;
    mem_context_init(&mctx);

    /* Load the module. */
    struct module *mod = NULL;
    struct load_context lctx;
    load_context_init(&lctx, &mctx);
    int ret = module_create(&mod, wasm_bytes, wasm_bytes + wasm_len, &lctx);
    if (ret != 0) {
        fprintf(stderr, "runner-toywasm: module_create: %d: %s\n",
                ret, report_getmessage(&lctx.report));
        return 1;
    }
    load_context_clear(&lctx);

    /* Locate _start. */
    uint32_t funcidx;
    struct name start_name = NAME_FROM_CSTR_LITERAL("_start");
    if (module_find_export(mod, &start_name, EXTERNTYPE_FUNC, &funcidx) != 0)
        die("module has no _start export");
    const struct functype *ft = module_functype(mod, funcidx);
    const struct resulttype *pt = &ft->parameter;
    const struct resulttype *rt = &ft->result;

    /* Create the WASI instance, inheriting host stdio fds 0/1/2. */
    struct wasi_instance *wasi = NULL;
    if (wasi_instance_create(&mctx, &wasi) != 0)
        die("wasi_instance_create failed");
    const char *wasi_argv[] = { argv[1] };
    wasi_instance_set_args(wasi, 1, wasi_argv);
    if (wasi_instance_populate_stdio_with_hostfd(wasi) != 0)
        die("wasi_instance_populate_stdio_with_hostfd failed");

    struct import_object *wasi_import = NULL;
    if (import_object_create_for_wasi(&mctx, wasi, &wasi_import) != 0)
        die("import_object_create_for_wasi failed");

    /* Instantiate. */
    struct report report;
    report_init(&report);
    struct instance *inst = NULL;
    ret = instance_create(&mctx, mod, &inst, wasi_import, &report);
    if (ret != 0) {
        fprintf(stderr, "runner-toywasm: instance_create: %d: %s\n",
                ret, report_getmessage(&report));
        return 1;
    }
    report_clear(&report);
    wasi_instance_set_memory(wasi, cconv_memory(inst));

    /* Execute _start. A clean WASI proc_exit surfaces as a voluntary-exit
     * trap; only a non-zero WASI exit code is a real failure. */
    struct exec_context ectx;
    exec_context_init(&ectx, inst, &mctx);
    ret = instance_execute_func(&ectx, funcidx, pt, rt);
    ret = instance_execute_handle_restart(&ectx, ret);
    uint32_t wasi_exit = 0;
    int rc = 0;
    if (ret == ETOYWASMTRAP) {
        const struct trap_info *trap = &ectx.trap;
        if (trap->trapid == TRAP_VOLUNTARY_EXIT) {
            wasi_exit = wasi_instance_exit_code(wasi);
            rc = (int)wasi_exit;
        } else {
            fprintf(stderr, "runner-toywasm: trap %u: %s\n",
                    (unsigned int)trap->trapid, report_getmessage(ectx.report));
            rc = 1;
        }
    } else if (ret != 0) {
        fprintf(stderr, "runner-toywasm: execute: %d\n", ret);
        rc = 1;
    }
    exec_context_clear(&ectx);

    fflush(stdout);
    instance_destroy(inst);
    import_object_destroy(&mctx, wasi_import);
    wasi_instance_destroy(wasi);
    module_destroy(&mctx, mod);
    mem_context_clear(&mctx);
    free(wasm_bytes);
    return rc;
}
