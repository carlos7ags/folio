/*
 * Folio C ABI — PDF generation library
 * https://github.com/carlos7ags/folio
 * Apache 2.0 License
 *
 * MEMORY OWNERSHIP RULES
 * ----------------------
 * 1. All objects are opaque uint64_t handles. The library owns the memory.
 *    Every folio_*_new() / folio_*_load() has a matching folio_*_free().
 *
 * 2. Strings passed TO the library (const char*) are copied immediately.
 *    The caller retains ownership and may free after the call returns.
 *
 * 3. Strings returned FROM the library:
 *    - folio_version(): persistent pointer, do NOT free.
 *    - folio_last_error(): library-owned, valid until the next C ABI call.
 *    - All other string data is returned as buffer handles (see below).
 *
 * 4. Buffer handles (folio_buffer_data / folio_buffer_len / folio_buffer_free):
 *    The library allocates the buffer; the caller MUST call folio_buffer_free()
 *    when done. The data pointer is valid until folio_buffer_free() is called.
 *
 * ERROR CONVENTION
 * ----------------
 * Functions returning int32_t: 0 = success, negative = error.
 * Call folio_last_error() for the human-readable message.
 *
 * THREAD SAFETY
 * -------------
 * The library is NOT thread-safe. All calls must be serialized.
 * A single-threaded wrapper is recommended for multi-threaded applications.
 */

#ifndef FOLIO_H
#define FOLIO_H

#include <stdint.h>
#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

/* ── Error codes ───────────────────────────────────────────────────── */

#define FOLIO_OK               0
#define FOLIO_ERR_HANDLE      -1   /* invalid or expired handle */
#define FOLIO_ERR_ARG         -2   /* invalid argument (NULL, out of range) */
#define FOLIO_ERR_IO          -3   /* file I/O error */
#define FOLIO_ERR_PDF         -4   /* PDF generation/parsing error */
#define FOLIO_ERR_TYPE        -5   /* handle type mismatch */
#define FOLIO_ERR_INTERNAL    -6   /* unexpected internal error */

/* ── Enums ─────────────────────────────────────────────────────────── */

/* Text alignment (layout.Align) */
#define FOLIO_ALIGN_LEFT       0
#define FOLIO_ALIGN_CENTER     1
#define FOLIO_ALIGN_RIGHT      2
#define FOLIO_ALIGN_JUSTIFY    3

/* Vertical alignment (layout.VAlign) */
#define FOLIO_VALIGN_TOP       0
#define FOLIO_VALIGN_MIDDLE    1
#define FOLIO_VALIGN_BOTTOM    2

/* Heading levels (layout.HeadingLevel) */
#define FOLIO_H1               1
#define FOLIO_H2               2
#define FOLIO_H3               3
#define FOLIO_H4               4
#define FOLIO_H5               5
#define FOLIO_H6               6

/* List styles (layout.ListStyle) */
#define FOLIO_LIST_BULLET      0
#define FOLIO_LIST_DECIMAL     1
#define FOLIO_LIST_LOWER_ALPHA 2
#define FOLIO_LIST_UPPER_ALPHA 3
#define FOLIO_LIST_LOWER_ROMAN 4
#define FOLIO_LIST_UPPER_ROMAN 5

/* PDF/A levels (document.PdfALevel) */
#define FOLIO_PDFA_2B          0
#define FOLIO_PDFA_2U          1
#define FOLIO_PDFA_2A          2
#define FOLIO_PDFA_3B          3
#define FOLIO_PDFA_1B          4
#define FOLIO_PDFA_1A          5

/* Encryption algorithms (document.EncryptionAlgorithm) */
#define FOLIO_ENCRYPT_RC4_128  0
#define FOLIO_ENCRYPT_AES_128  1
#define FOLIO_ENCRYPT_AES_256  2

/* QR error correction levels (barcode.ECCLevel) */
#define FOLIO_ECC_L            0   /* 7% recovery */
#define FOLIO_ECC_M            1   /* 15% recovery */
#define FOLIO_ECC_Q            2   /* 25% recovery */
#define FOLIO_ECC_H            3   /* 30% recovery */

/* Flex direction (layout.FlexDirection) */
#define FOLIO_FLEX_ROW         0
#define FOLIO_FLEX_COLUMN      1

/* Flex justify-content (layout.JustifyContent) */
#define FOLIO_JUSTIFY_START         0
#define FOLIO_JUSTIFY_END           1
#define FOLIO_JUSTIFY_CENTER        2
#define FOLIO_JUSTIFY_SPACE_BETWEEN 3
#define FOLIO_JUSTIFY_SPACE_AROUND  4
#define FOLIO_JUSTIFY_SPACE_EVENLY  5

/* Flex align-items / align-self (layout.AlignItems) */
#define FOLIO_CROSS_STRETCH    0
#define FOLIO_CROSS_START      1
#define FOLIO_CROSS_END        2
#define FOLIO_CROSS_CENTER     3

/* Flex wrap (layout.FlexWrap) */
#define FOLIO_FLEX_NOWRAP      0
#define FOLIO_FLEX_WRAP        1

/* ── Callback types ────────────────────────────────────────────────── */

/*
 * Page decorator callback. Invoked for each page during rendering.
 * page_handle is a temporary handle valid only inside the callback.
 */
typedef void (*folio_page_decorator_fn)(
    int32_t   page_index,
    int32_t   total_pages,
    uint64_t  page_handle,
    void     *user_data
);

/* ── Core ──────────────────────────────────────────────────────────── */

const char *folio_version(void);
const char *folio_last_error(void);

/* ── Buffer ────────────────────────────────────────────────────────── */

void    *folio_buffer_data(uint64_t buf);
int32_t  folio_buffer_len(uint64_t buf);
void     folio_buffer_free(uint64_t buf);

/* ── Document ──────────────────────────────────────────────────────── */

uint64_t folio_document_new(double width, double height);
uint64_t folio_document_new_letter(void);
uint64_t folio_document_new_a4(void);
void     folio_document_free(uint64_t doc);

int32_t  folio_document_set_title(uint64_t doc, const char *title);
int32_t  folio_document_set_author(uint64_t doc, const char *author);
int32_t  folio_document_set_margins(uint64_t doc, double top, double right, double bottom, double left);

uint64_t folio_document_add_page(uint64_t doc);
int32_t  folio_document_page_count(uint64_t doc);
int32_t  folio_document_add(uint64_t doc, uint64_t element);

int32_t  folio_document_save(uint64_t doc, const char *path);
uint64_t folio_document_write_to_buffer(uint64_t doc);

/* Document features */
int32_t  folio_document_set_tagged(uint64_t doc, int32_t enabled);
int32_t  folio_document_set_pdfa(uint64_t doc, int32_t level);
int32_t  folio_document_set_encryption(uint64_t doc, const char *user_pw, const char *owner_pw, int32_t algorithm);
int32_t  folio_document_set_auto_bookmarks(uint64_t doc, int32_t enabled);
int32_t  folio_document_set_form(uint64_t doc, uint64_t form);

/* Callbacks */
int32_t  folio_document_set_header(uint64_t doc, folio_page_decorator_fn fn, void *user_data);
int32_t  folio_document_set_footer(uint64_t doc, folio_page_decorator_fn fn, void *user_data);

/* Watermark */
int32_t  folio_document_set_watermark(uint64_t doc, const char *text);
int32_t  folio_document_set_watermark_config(uint64_t doc, const char *text,
             double font_size, double color_r, double color_g, double color_b,
             double angle, double opacity);

/* Outlines / Bookmarks */
uint64_t folio_document_add_outline(uint64_t doc, const char *title, int32_t page_index);
uint64_t folio_document_add_outline_xyz(uint64_t doc, const char *title,
             int32_t page_index, double left, double top, double zoom);
uint64_t folio_outline_add_child(uint64_t outline, const char *title, int32_t page_index);
uint64_t folio_outline_add_child_xyz(uint64_t outline, const char *title,
             int32_t page_index, double left, double top, double zoom);

/* Named Destinations */
int32_t  folio_document_add_named_dest(uint64_t doc, const char *name, int32_t page_index,
             const char *fit_type, double top, double left, double zoom);

/* Viewer Preferences */
int32_t  folio_document_set_viewer_preferences(uint64_t doc,
             const char *page_layout, const char *page_mode,
             int32_t hide_toolbar, int32_t hide_menubar, int32_t hide_window_ui,
             int32_t fit_window, int32_t center_window, int32_t display_doc_title);

/* Page Labels */
int32_t  folio_document_add_page_label(uint64_t doc, int32_t page_index,
             const char *style, const char *prefix, int32_t start);

/* Page management */
int32_t  folio_document_remove_page(uint64_t doc, int32_t index);

/* Absolute positioning */
int32_t  folio_document_add_absolute(uint64_t doc, uint64_t element, double x, double y, double width);

/* File attachments (PDF/A-3b) */
int32_t  folio_document_attach_file(uint64_t doc, const void *data, int32_t length,
             const char *file_name, const char *mime_type, const char *description, const char *af_relationship);

/* Inline HTML */
int32_t  folio_document_add_html(uint64_t doc, const char *html);
int32_t  folio_document_add_html_with_options(uint64_t doc, const char *html,
             double default_font_size, double page_width, double page_height,
             const char *base_path, const char *fallback_font_path);

/* Page-specific margins (@page :first/:left/:right) */
int32_t  folio_document_set_first_margins(uint64_t doc, double top, double right, double bottom, double left);
int32_t  folio_document_set_left_margins(uint64_t doc, double top, double right, double bottom, double left);
int32_t  folio_document_set_right_margins(uint64_t doc, double top, double right, double bottom, double left);

/* ── Page ──────────────────────────────────────────────────────────── */

int32_t  folio_page_add_text(uint64_t page, const char *text, uint64_t font, double size, double x, double y);
int32_t  folio_page_add_text_embedded(uint64_t page, const char *text, uint64_t font, double size, double x, double y);
int32_t  folio_page_add_image(uint64_t page, uint64_t img, double x, double y, double w, double h);
int32_t  folio_page_add_link(uint64_t page, double x1, double y1, double x2, double y2, const char *uri);
int32_t  folio_page_add_internal_link(uint64_t page, double x1, double y1, double x2, double y2, const char *dest_name);
int32_t  folio_page_add_text_annotation(uint64_t page, double x1, double y1, double x2, double y2, const char *text, const char *icon);
int32_t  folio_page_set_opacity(uint64_t page, double alpha);
int32_t  folio_page_set_rotate(uint64_t page, int32_t degrees);
int32_t  folio_page_set_crop_box(uint64_t page, double x1, double y1, double x2, double y2);
int32_t  folio_page_set_trim_box(uint64_t page, double x1, double y1, double x2, double y2);
int32_t  folio_page_set_bleed_box(uint64_t page, double x1, double y1, double x2, double y2);
int32_t  folio_page_set_art_box(uint64_t page, double x1, double y1, double x2, double y2);
int32_t  folio_page_set_size(uint64_t page, double width, double height);
int32_t  folio_page_add_page_link(uint64_t page, double x1, double y1, double x2, double y2, int32_t target_page);
int32_t  folio_page_set_opacity_fill_stroke(uint64_t page, double fill_alpha, double stroke_alpha);
int32_t  folio_page_add_highlight(uint64_t page, double x1, double y1, double x2, double y2,
             double r, double g, double b, const double *quad_points, int32_t quad_count);
int32_t  folio_page_add_underline_annotation(uint64_t page, double x1, double y1, double x2, double y2,
             double r, double g, double b, const double *quad_points, int32_t quad_count);
int32_t  folio_page_add_squiggly(uint64_t page, double x1, double y1, double x2, double y2,
             double r, double g, double b, const double *quad_points, int32_t quad_count);
int32_t  folio_page_add_strikeout(uint64_t page, double x1, double y1, double x2, double y2,
             double r, double g, double b, const double *quad_points, int32_t quad_count);

/* ── Font ──────────────────────────────────────────────────────────── */

uint64_t folio_font_standard(const char *name);
uint64_t folio_font_helvetica(void);
uint64_t folio_font_helvetica_bold(void);
uint64_t folio_font_helvetica_oblique(void);
uint64_t folio_font_helvetica_bold_oblique(void);
uint64_t folio_font_times_roman(void);
uint64_t folio_font_times_bold(void);
uint64_t folio_font_times_italic(void);
uint64_t folio_font_times_bold_italic(void);
uint64_t folio_font_courier(void);
uint64_t folio_font_courier_bold(void);
uint64_t folio_font_courier_oblique(void);
uint64_t folio_font_courier_bold_oblique(void);
uint64_t folio_font_symbol(void);
uint64_t folio_font_zapf_dingbats(void);

uint64_t folio_font_load_ttf(const char *path);
uint64_t folio_font_parse_ttf(const void *data, int32_t length);
void     folio_font_free(uint64_t font);

/* ── Paragraph ─────────────────────────────────────────────────────── */

uint64_t folio_paragraph_new(const char *text, uint64_t font, double font_size);
uint64_t folio_paragraph_new_embedded(const char *text, uint64_t font, double font_size);
void     folio_paragraph_free(uint64_t para);

int32_t  folio_paragraph_set_align(uint64_t para, int32_t align);
int32_t  folio_paragraph_set_leading(uint64_t para, double leading);
int32_t  folio_paragraph_set_space_before(uint64_t para, double pts);
int32_t  folio_paragraph_set_space_after(uint64_t para, double pts);
int32_t  folio_paragraph_set_background(uint64_t para, double r, double g, double b);
int32_t  folio_paragraph_set_first_indent(uint64_t para, double pts);
int32_t  folio_paragraph_set_orphans(uint64_t para, int32_t n);
int32_t  folio_paragraph_set_widows(uint64_t para, int32_t n);
int32_t  folio_paragraph_set_ellipsis(uint64_t para, int32_t enabled);
int32_t  folio_paragraph_set_word_break(uint64_t para, const char *mode);
int32_t  folio_paragraph_set_hyphens(uint64_t para, const char *mode);

int32_t  folio_paragraph_add_run(uint64_t para, const char *text, uint64_t font, double font_size, double r, double g, double b);

/* ── Heading ───────────────────────────────────────────────────────── */

uint64_t folio_heading_new(const char *text, int32_t level);
uint64_t folio_heading_new_with_font(const char *text, int32_t level, uint64_t font, double font_size);
uint64_t folio_heading_new_embedded(const char *text, int32_t level, uint64_t font);
void     folio_heading_free(uint64_t heading);

int32_t  folio_heading_set_align(uint64_t heading, int32_t align);

/* ── Table ─────────────────────────────────────────────────────────── */

uint64_t folio_table_new(void);
void     folio_table_free(uint64_t table);

int32_t  folio_table_set_column_widths(uint64_t table, const double *widths, int32_t count);
int32_t  folio_table_set_border_collapse(uint64_t table, int32_t enabled);
int32_t  folio_table_set_cell_spacing(uint64_t table, double h, double v);
int32_t  folio_table_set_auto_column_widths(uint64_t table);
int32_t  folio_table_set_min_width(uint64_t table, double pts);

uint64_t folio_table_add_row(uint64_t table);
uint64_t folio_table_add_header_row(uint64_t table);
uint64_t folio_table_add_footer_row(uint64_t table);
void     folio_row_free(uint64_t row);

uint64_t folio_row_add_cell(uint64_t row, const char *text, uint64_t font, double font_size);
uint64_t folio_row_add_cell_embedded(uint64_t row, const char *text, uint64_t font, double font_size);
uint64_t folio_row_add_cell_element(uint64_t row, uint64_t element);
void     folio_cell_free(uint64_t cell);

int32_t  folio_cell_set_align(uint64_t cell, int32_t align);
int32_t  folio_cell_set_padding(uint64_t cell, double padding);
int32_t  folio_cell_set_padding_sides(uint64_t cell, double top, double right, double bottom, double left);
int32_t  folio_cell_set_valign(uint64_t cell, int32_t valign);
int32_t  folio_cell_set_background(uint64_t cell, double r, double g, double b);
int32_t  folio_cell_set_colspan(uint64_t cell, int32_t n);
int32_t  folio_cell_set_rowspan(uint64_t cell, int32_t n);
int32_t  folio_cell_set_border(uint64_t cell, double width, double r, double g, double b);
int32_t  folio_cell_set_borders(uint64_t cell,
             double top_w, double top_r, double top_g, double top_b,
             double right_w, double right_r, double right_g, double right_b,
             double bottom_w, double bottom_r, double bottom_g, double bottom_b,
             double left_w, double left_r, double left_g, double left_b);
int32_t  folio_cell_set_width_hint(uint64_t cell, double pts);

/* ── Image ─────────────────────────────────────────────────────────── */

uint64_t folio_image_load_jpeg(const char *path);
uint64_t folio_image_load_png(const char *path);
uint64_t folio_image_load_tiff(const char *path);
uint64_t folio_image_parse_jpeg(const void *data, int32_t length);
uint64_t folio_image_parse_png(const void *data, int32_t length);
int32_t  folio_image_width(uint64_t img);
int32_t  folio_image_height(uint64_t img);
void     folio_image_free(uint64_t img);

uint64_t folio_image_element_new(uint64_t img);
int32_t  folio_image_element_set_size(uint64_t elem, double w, double h);
int32_t  folio_image_element_set_align(uint64_t elem, int32_t align);
void     folio_image_element_free(uint64_t elem);

/* ── Div (container) ───────────────────────────────────────────────── */

uint64_t folio_div_new(void);
void     folio_div_free(uint64_t div);

int32_t  folio_div_add(uint64_t div, uint64_t element);
int32_t  folio_div_set_padding(uint64_t div, double top, double right, double bottom, double left);
int32_t  folio_div_set_background(uint64_t div, double r, double g, double b);
int32_t  folio_div_set_border(uint64_t div, double width, double r, double g, double b);
int32_t  folio_div_set_width(uint64_t div, double pts);
int32_t  folio_div_set_min_height(uint64_t div, double pts);
int32_t  folio_div_set_max_width(uint64_t div, double pts);
int32_t  folio_div_set_min_width(uint64_t div, double pts);
int32_t  folio_div_set_space_before(uint64_t div, double pts);
int32_t  folio_div_set_space_after(uint64_t div, double pts);
int32_t  folio_div_set_border_radius(uint64_t div, double radius);
int32_t  folio_div_set_opacity(uint64_t div, double opacity);
int32_t  folio_div_set_overflow(uint64_t div, const char *mode);
int32_t  folio_div_set_box_shadow(uint64_t div, double offset_x, double offset_y,
             double blur, double spread, double r, double g, double b);
int32_t  folio_div_set_max_height(uint64_t div, double pts);

/* ── List ──────────────────────────────────────────────────────────── */

uint64_t folio_list_new(uint64_t font, double font_size);
uint64_t folio_list_new_embedded(uint64_t font, double font_size);
void     folio_list_free(uint64_t list);

int32_t  folio_list_set_style(uint64_t list, int32_t style);
int32_t  folio_list_set_indent(uint64_t list, double indent);
int32_t  folio_list_set_leading(uint64_t list, double leading);
int32_t  folio_list_add_item(uint64_t list, const char *text);
uint64_t folio_list_add_nested_item(uint64_t list, const char *text);

/* ── Link (layout element) ────────────────────────────────────────── */

uint64_t folio_link_new(const char *text, const char *uri, uint64_t font, double font_size);
uint64_t folio_link_new_embedded(const char *text, const char *uri, uint64_t font, double font_size);
uint64_t folio_link_new_internal(const char *text, const char *dest_name, uint64_t font, double font_size);
void     folio_link_free(uint64_t link);

int32_t  folio_link_set_color(uint64_t link, double r, double g, double b);
int32_t  folio_link_set_underline(uint64_t link);
int32_t  folio_link_set_align(uint64_t link, int32_t align);

/* ── TextRun builder (styled text for headings, lists) ─────────────── */

/* Decoration constants */
#define FOLIO_DECORATION_NONE           0
#define FOLIO_DECORATION_UNDERLINE      1
#define FOLIO_DECORATION_STRIKETHROUGH  2

uint64_t folio_run_list_new(void);
void     folio_run_list_free(uint64_t rl);

int32_t  folio_run_list_add(uint64_t rl, const char *text, uint64_t font, double font_size, double r, double g, double b);
int32_t  folio_run_list_add_embedded(uint64_t rl, const char *text, uint64_t font, double font_size, double r, double g, double b);
int32_t  folio_run_list_add_link(uint64_t rl, const char *text, uint64_t font, double font_size,
             double r, double g, double b, const char *uri, int32_t underline);
int32_t  folio_run_list_last_set_underline(uint64_t rl);
int32_t  folio_run_list_last_set_strikethrough(uint64_t rl);
int32_t  folio_run_list_last_set_letter_spacing(uint64_t rl, double spacing);

int32_t  folio_heading_set_runs(uint64_t heading, uint64_t run_list);
int32_t  folio_list_add_item_runs(uint64_t list, uint64_t run_list);
uint64_t folio_list_add_item_runs_with_sublist(uint64_t list, uint64_t run_list);

/* ── Misc layout elements ──────────────────────────────────────────── */

uint64_t folio_line_separator_new(void);
uint64_t folio_area_break_new(void);

/* ── Barcode ──────────────────────────────────────────────────────── */

uint64_t folio_barcode_qr(const char *data);
uint64_t folio_barcode_qr_ecc(const char *data, int32_t level);
uint64_t folio_barcode_code128(const char *data);
uint64_t folio_barcode_ean13(const char *data);
int32_t  folio_barcode_width(uint64_t bc);
int32_t  folio_barcode_height(uint64_t bc);
void     folio_barcode_free(uint64_t bc);

uint64_t folio_barcode_element_new(uint64_t bc, double width);
int32_t  folio_barcode_element_set_height(uint64_t elem, double height);
int32_t  folio_barcode_element_set_align(uint64_t elem, int32_t align);
void     folio_barcode_element_free(uint64_t elem);

/* ── SVG ──────────────────────────────────────────────────────────── */

uint64_t folio_svg_parse(const char *svg_xml);
uint64_t folio_svg_parse_bytes(const void *data, int32_t length);
double   folio_svg_width(uint64_t svg);
double   folio_svg_height(uint64_t svg);
void     folio_svg_free(uint64_t svg);

uint64_t folio_svg_element_new(uint64_t svg);
int32_t  folio_svg_element_set_size(uint64_t elem, double w, double h);
int32_t  folio_svg_element_set_align(uint64_t elem, int32_t align);
void     folio_svg_element_free(uint64_t elem);

/* ── Flex (container) ─────────────────────────────────────────────── */

uint64_t folio_flex_new(void);
void     folio_flex_free(uint64_t flex);

int32_t  folio_flex_add(uint64_t flex, uint64_t element);
int32_t  folio_flex_add_item(uint64_t flex, uint64_t item);
int32_t  folio_flex_set_direction(uint64_t flex, int32_t direction);
int32_t  folio_flex_set_justify_content(uint64_t flex, int32_t justify);
int32_t  folio_flex_set_align_items(uint64_t flex, int32_t align);
int32_t  folio_flex_set_wrap(uint64_t flex, int32_t wrap);
int32_t  folio_flex_set_gap(uint64_t flex, double gap);
int32_t  folio_flex_set_padding(uint64_t flex, double padding);
int32_t  folio_flex_set_background(uint64_t flex, double r, double g, double b);
int32_t  folio_flex_set_space_before(uint64_t flex, double pts);
int32_t  folio_flex_set_space_after(uint64_t flex, double pts);
int32_t  folio_flex_set_row_gap(uint64_t flex, double gap);
int32_t  folio_flex_set_column_gap(uint64_t flex, double gap);
int32_t  folio_flex_set_padding_all(uint64_t flex, double top, double right, double bottom, double left);
int32_t  folio_flex_set_border(uint64_t flex, double width, double r, double g, double b);

uint64_t folio_flex_item_new(uint64_t element);
void     folio_flex_item_free(uint64_t item);

int32_t  folio_flex_item_set_grow(uint64_t item, double grow);
int32_t  folio_flex_item_set_shrink(uint64_t item, double shrink);
int32_t  folio_flex_item_set_basis(uint64_t item, double basis);
int32_t  folio_flex_item_set_align_self(uint64_t item, int32_t align);
int32_t  folio_flex_item_set_margins(uint64_t item, double top, double right, double bottom, double left);

/* ── HTML to PDF ───────────────────────────────────────────────────── */

int32_t  folio_html_to_pdf(const char *html, const char *output_path);
uint64_t folio_html_to_buffer(const char *html, double page_width, double page_height);
uint64_t folio_html_convert(const char *html, double page_width, double page_height);

/* ── Reader (PDF parsing) ──────────────────────────────────────────── */

uint64_t folio_reader_open(const char *path);
uint64_t folio_reader_parse(const void *data, int32_t length);
void     folio_reader_free(uint64_t reader);

int32_t  folio_reader_page_count(uint64_t reader);
uint64_t folio_reader_version(uint64_t reader);
uint64_t folio_reader_info_title(uint64_t reader);
uint64_t folio_reader_info_author(uint64_t reader);
uint64_t folio_reader_extract_text(uint64_t reader, int32_t page_index);
double   folio_reader_page_width(uint64_t reader, int32_t page_index);
double   folio_reader_page_height(uint64_t reader, int32_t page_index);

/* ── Merge (combine PDFs) ──────────────────────────────────────────── */

uint64_t folio_reader_merge(const uint64_t *readers, int32_t count);
uint64_t folio_merge_files(const char **paths, int32_t count);
int32_t  folio_merge_set_info(uint64_t merged, const char *title, const char *author);
int32_t  folio_merge_add_blank_page(uint64_t merged, double width, double height);
int32_t  folio_merge_add_page_with_text(uint64_t merged, double width, double height,
             const char *text, uint64_t font, double font_size, double x, double y);
int32_t  folio_merge_save(uint64_t merged, const char *path);
uint64_t folio_merge_write_to_buffer(uint64_t merged);
void     folio_merge_free(uint64_t merged);

/* ── Forms ─────────────────────────────────────────────────────────── */

uint64_t folio_form_new(void);
void     folio_form_free(uint64_t form);

int32_t  folio_form_add_text_field(uint64_t form, const char *name, double x1, double y1, double x2, double y2, int32_t page_index);
int32_t  folio_form_add_checkbox(uint64_t form, const char *name, double x1, double y1, double x2, double y2, int32_t page_index, int32_t checked);
int32_t  folio_form_add_dropdown(uint64_t form, const char *name, double x1, double y1, double x2, double y2, int32_t page_index, const char **options, int32_t opt_count);
int32_t  folio_form_add_signature(uint64_t form, const char *name, double x1, double y1, double x2, double y2, int32_t page_index);
int32_t  folio_form_add_multiline_text_field(uint64_t form, const char *name, double x1, double y1, double x2, double y2, int32_t page_index);
int32_t  folio_form_add_password_field(uint64_t form, const char *name, double x1, double y1, double x2, double y2, int32_t page_index);
int32_t  folio_form_add_listbox(uint64_t form, const char *name, double x1, double y1, double x2, double y2, int32_t page_index, const char **options, int32_t opt_count);
int32_t  folio_form_add_radio_group(uint64_t form, const char *name,
             const char **values, const double *rects, const int32_t *page_indices, int32_t count);

/* Form field builder — create field, configure, then add to form */
uint64_t folio_form_create_text_field(const char *name, double x1, double y1, double x2, double y2, int32_t page_index);
uint64_t folio_form_create_checkbox(const char *name, double x1, double y1, double x2, double y2, int32_t page_index, int32_t checked);
int32_t  folio_form_add_field(uint64_t form, uint64_t field);
void     folio_form_field_free(uint64_t field);

int32_t  folio_form_field_set_value(uint64_t field, const char *value);
int32_t  folio_form_field_set_read_only(uint64_t field);
int32_t  folio_form_field_set_required(uint64_t field);
int32_t  folio_form_field_set_background_color(uint64_t field, double r, double g, double b);
int32_t  folio_form_field_set_border_color(uint64_t field, double r, double g, double b);

/* Form filling (modify existing PDF forms) */
uint64_t folio_form_filler_new(uint64_t reader);
void     folio_form_filler_free(uint64_t filler);
uint64_t folio_form_filler_field_names(uint64_t filler);     /* returns buffer handle (newline-separated) */
uint64_t folio_form_filler_get_value(uint64_t filler, const char *field_name);  /* returns buffer handle */
int32_t  folio_form_filler_set_value(uint64_t filler, const char *field_name, const char *value);
int32_t  folio_form_filler_set_checkbox(uint64_t filler, const char *field_name, int32_t checked);

/* ── Grid (container) ─────────────────────────────────────────────── */

/* Grid track types: 0=px, 1=percent, 2=fr, 3=auto */
#define FOLIO_GRID_PX          0
#define FOLIO_GRID_PERCENT     1
#define FOLIO_GRID_FR          2
#define FOLIO_GRID_AUTO        3

/* Float sides */
#define FOLIO_FLOAT_LEFT       0
#define FOLIO_FLOAT_RIGHT      1

/* Tab alignment */
#define FOLIO_TAB_LEFT         0
#define FOLIO_TAB_RIGHT        1
#define FOLIO_TAB_CENTER       2

uint64_t folio_grid_new(void);
void     folio_grid_free(uint64_t grid);

int32_t  folio_grid_add_child(uint64_t grid, uint64_t element);
int32_t  folio_grid_set_template_columns(uint64_t grid, const int32_t *types, const double *values, int32_t count);
int32_t  folio_grid_set_template_rows(uint64_t grid, const int32_t *types, const double *values, int32_t count);
int32_t  folio_grid_set_auto_rows(uint64_t grid, const int32_t *types, const double *values, int32_t count);
int32_t  folio_grid_set_gap(uint64_t grid, double row_gap, double col_gap);
int32_t  folio_grid_set_placement(uint64_t grid, int32_t child_index, int32_t col_start, int32_t col_end, int32_t row_start, int32_t row_end);
int32_t  folio_grid_set_padding(uint64_t grid, double padding);
int32_t  folio_grid_set_background(uint64_t grid, double r, double g, double b);
int32_t  folio_grid_set_justify_items(uint64_t grid, int32_t align);
int32_t  folio_grid_set_align_items(uint64_t grid, int32_t align);
int32_t  folio_grid_set_justify_content(uint64_t grid, int32_t justify);
int32_t  folio_grid_set_align_content(uint64_t grid, int32_t align);
int32_t  folio_grid_set_space_before(uint64_t grid, double pts);
int32_t  folio_grid_set_space_after(uint64_t grid, double pts);

/* ── Columns (multi-column layout) ────────────────────────────────── */

uint64_t folio_columns_new(int32_t cols);
void     folio_columns_free(uint64_t columns);

int32_t  folio_columns_set_gap(uint64_t columns, double gap);
int32_t  folio_columns_set_widths(uint64_t columns, const double *widths, int32_t count);
int32_t  folio_columns_add(uint64_t columns, int32_t col_index, uint64_t element);

/* ── Float (text wrapping) ────────────────────────────────────────── */

uint64_t folio_float_new(int32_t side, uint64_t element);  /* FOLIO_FLOAT_LEFT/RIGHT */
void     folio_float_free(uint64_t flt);
int32_t  folio_float_set_margin(uint64_t flt, double margin);

/* ── TabbedLine (tab-stop text) ───────────────────────────────────── */

uint64_t folio_tabbed_line_new(uint64_t font, double font_size,
             const double *positions, const int32_t *aligns, const int32_t *leaders, int32_t count);
uint64_t folio_tabbed_line_new_embedded(uint64_t font, double font_size,
             const double *positions, const int32_t *aligns, const int32_t *leaders, int32_t count);
void     folio_tabbed_line_free(uint64_t tl);

int32_t  folio_tabbed_line_set_segments(uint64_t tl, const char **segments, int32_t count);
int32_t  folio_tabbed_line_set_color(uint64_t tl, double r, double g, double b);
int32_t  folio_tabbed_line_set_leading(uint64_t tl, double leading);

#ifdef __cplusplus
}
#endif

#endif /* FOLIO_H */
