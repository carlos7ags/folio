/*
 * C integration test for the Folio C ABI.
 * Compile and run:
 *   cc -o test_cabi test_cabi.c -L../.. -lfolio -Wl,-rpath,../..
 *   ./test_cabi
 */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

/* Use the auto-generated header from the build */
#include "../../libfolio.h"

#define ASSERT(cond, msg) do { \
    if (!(cond)) { \
        fprintf(stderr, "FAIL: %s (line %d)\n", msg, __LINE__); \
        const char* err = folio_last_error(); \
        if (err) fprintf(stderr, "  last_error: %s\n", err); \
        failures++; \
    } else { \
        passes++; \
    } \
} while(0)

int main(void) {
    int passes = 0, failures = 0;

    /* Test 1: Version string — persistent pointer, do NOT free */
    const char* ver = folio_version();
    ASSERT(ver != NULL, "folio_version returns non-null");
    ASSERT(strlen(ver) > 0, "version string is non-empty");
    printf("folio version: %s\n", ver);

    /* Test 2: Create blank document and save to buffer */
    uint64_t doc = folio_document_new_letter();
    ASSERT(doc != 0, "document_new_letter returns handle");

    int32_t rc = folio_document_set_title(doc, "C ABI Test");
    ASSERT(rc == 0, "set_title succeeds");

    rc = folio_document_set_author(doc, "Test Author");
    ASSERT(rc == 0, "set_author succeeds");

    rc = folio_document_set_margins(doc, 36, 36, 36, 36);
    ASSERT(rc == 0, "set_margins succeeds");

    uint64_t page = folio_document_add_page(doc);
    ASSERT(page != 0, "add_page returns handle");

    int32_t count = folio_document_page_count(doc);
    ASSERT(count == 1, "page_count is 1");

    uint64_t buf = folio_document_write_to_buffer(doc);
    ASSERT(buf != 0, "write_to_buffer returns handle");

    int32_t len = folio_buffer_len(buf);
    ASSERT(len > 0, "buffer has data");

    void* data = folio_buffer_data(buf);
    ASSERT(data != NULL, "buffer data is non-null");
    ASSERT(memcmp(data, "%PDF-1.7", 8) == 0, "buffer starts with PDF header");

    folio_buffer_free(buf);
    folio_document_free(doc);

    /* Test 3: Text on page with standard font */
    doc = folio_document_new(595.28, 841.89); /* A4 */
    ASSERT(doc != 0, "document_new with custom size");

    folio_document_set_title(doc, "Font Test");

    page = folio_document_add_page(doc);
    ASSERT(page != 0, "add_page for font test");

    uint64_t helv = folio_font_helvetica();
    ASSERT(helv != 0, "font_helvetica returns handle");

    rc = folio_page_add_text(page, "Hello from C!", helv, 24.0, 72.0, 700.0);
    ASSERT(rc == 0, "page_add_text succeeds");

    uint64_t times = folio_font_times_roman();
    ASSERT(times != 0, "font_times_roman returns handle");

    rc = folio_page_add_text(page, "Second line in Times.", times, 12.0, 72.0, 660.0);
    ASSERT(rc == 0, "page_add_text with Times succeeds");

    rc = folio_page_add_link(page, 72.0, 640.0, 200.0, 655.0, "https://folio.dev");
    ASSERT(rc == 0, "page_add_link succeeds");

    /* Save to file */
    rc = folio_document_save(doc, "/tmp/folio_cabi_test.pdf");
    ASSERT(rc == 0, "document_save succeeds");

    folio_document_free(doc);

    /* Test 4: Invalid handle */
    rc = folio_document_set_title(99999, "bad");
    ASSERT(rc != 0, "invalid handle returns error");
    ASSERT(folio_last_error() != NULL, "last_error set for invalid handle");

    /* Test 5: Font lookup by name */
    uint64_t courier = folio_font_standard("Courier");
    ASSERT(courier != 0, "font_standard Courier");

    uint64_t bad_font = folio_font_standard("NotAFont");
    ASSERT(bad_font == 0, "unknown font returns 0");

    /* Test 6: Layout engine — paragraphs with word wrapping */
    doc = folio_document_new_letter();
    folio_document_set_title(doc, "Layout Test");

    helv = folio_font_helvetica();
    uint64_t para = folio_paragraph_new("This is a paragraph that should wrap automatically when it exceeds the page width. The layout engine handles word wrapping and page breaks.", helv, 12.0);
    ASSERT(para != 0, "paragraph_new returns handle");

    rc = folio_paragraph_set_align(para, 0); /* AlignLeft */
    ASSERT(rc == 0, "paragraph_set_align succeeds");

    rc = folio_paragraph_set_leading(para, 1.5);
    ASSERT(rc == 0, "paragraph_set_leading succeeds");

    rc = folio_paragraph_set_space_after(para, 12.0);
    ASSERT(rc == 0, "paragraph_set_space_after succeeds");

    rc = folio_paragraph_set_background(para, 0.95, 0.95, 0.95);
    ASSERT(rc == 0, "paragraph_set_background succeeds");

    rc = folio_paragraph_set_first_indent(para, 24.0);
    ASSERT(rc == 0, "paragraph_set_first_indent succeeds");

    rc = folio_document_add(doc, para);
    ASSERT(rc == 0, "document_add paragraph succeeds");

    /* Test 7: Heading */
    uint64_t h1 = folio_heading_new("Chapter 1: Introduction", 1);
    ASSERT(h1 != 0, "heading_new returns handle");

    rc = folio_heading_set_align(h1, 1); /* AlignCenter */
    ASSERT(rc == 0, "heading_set_align succeeds");

    rc = folio_document_add(doc, h1);
    ASSERT(rc == 0, "document_add heading succeeds");

    /* Test 8: Heading with specific font */
    uint64_t h2 = folio_heading_new_with_font("Section 1.1", 2, folio_font_helvetica_bold(), 18.0);
    ASSERT(h2 != 0, "heading_new_with_font returns handle");

    rc = folio_document_add(doc, h2);
    ASSERT(rc == 0, "document_add heading_with_font succeeds");

    /* Test 9: Paragraph with mixed runs */
    uint64_t styled = folio_paragraph_new("Bold start: ", folio_font_helvetica_bold(), 12.0);
    ASSERT(styled != 0, "styled paragraph created");

    rc = folio_paragraph_add_run(styled, "normal continuation.", helv, 12.0, 0.0, 0.0, 0.0);
    ASSERT(rc == 0, "paragraph_add_run succeeds");

    rc = folio_paragraph_add_run(styled, " Red text.", helv, 12.0, 1.0, 0.0, 0.0);
    ASSERT(rc == 0, "paragraph_add_run with color succeeds");

    rc = folio_document_add(doc, styled);
    ASSERT(rc == 0, "document_add styled paragraph succeeds");

    /* Save layout document */
    rc = folio_document_save(doc, "/tmp/folio_cabi_layout.pdf");
    ASSERT(rc == 0, "document_save layout succeeds");

    folio_paragraph_free(para);
    folio_heading_free(h1);
    folio_heading_free(h2);
    folio_paragraph_free(styled);
    folio_document_free(doc);

    /* Test 10: Font free — standard fonts are no-op */
    folio_font_free(helv); /* should not crash */
    uint64_t helv_again = folio_font_helvetica();
    ASSERT(helv_again != 0, "standard font still available after free");

    /* ===== Stage 5: Tables ===== */
    doc = folio_document_new_letter();
    folio_document_set_title(doc, "Table Test");
    helv = folio_font_helvetica();

    uint64_t tbl = folio_table_new();
    ASSERT(tbl != 0, "table_new returns handle");

    rc = folio_table_set_border_collapse(tbl, 1);
    ASSERT(rc == 0, "table_set_border_collapse succeeds");

    /* Header row */
    uint64_t hrow = folio_table_add_header_row(tbl);
    ASSERT(hrow != 0, "table_add_header_row returns handle");

    uint64_t c1 = folio_row_add_cell(hrow, "Name", helv, 12.0);
    ASSERT(c1 != 0, "row_add_cell returns handle");
    folio_cell_set_background(c1, 0.9, 0.9, 0.9);

    uint64_t c2 = folio_row_add_cell(hrow, "Value", helv, 12.0);
    ASSERT(c2 != 0, "row_add_cell 2 returns handle");

    /* Data row */
    uint64_t drow = folio_table_add_row(tbl);
    ASSERT(drow != 0, "table_add_row returns handle");
    folio_row_add_cell(drow, "Folio", helv, 12.0);
    folio_row_add_cell(drow, "PDF Library", helv, 12.0);

    rc = folio_document_add(doc, tbl);
    ASSERT(rc == 0, "document_add table succeeds");

    rc = folio_document_save(doc, "/tmp/folio_cabi_table.pdf");
    ASSERT(rc == 0, "table document save succeeds");
    folio_table_free(tbl);
    folio_document_free(doc);

    /* ===== Stage 7: Containers (Div, List, AreaBreak) ===== */
    doc = folio_document_new_letter();
    folio_document_set_title(doc, "Container Test");
    helv = folio_font_helvetica();

    uint64_t div = folio_div_new();
    ASSERT(div != 0, "div_new returns handle");

    rc = folio_div_set_padding(div, 10, 10, 10, 10);
    ASSERT(rc == 0, "div_set_padding succeeds");

    rc = folio_div_set_background(div, 0.95, 0.95, 1.0);
    ASSERT(rc == 0, "div_set_background succeeds");

    rc = folio_div_set_border(div, 1.0, 0.0, 0.0, 0.5);
    ASSERT(rc == 0, "div_set_border succeeds");

    /* Add a paragraph inside the div */
    para = folio_paragraph_new("Content inside a div.", helv, 12.0);
    rc = folio_div_add(div, para);
    ASSERT(rc == 0, "div_add paragraph succeeds");

    rc = folio_document_add(doc, div);
    ASSERT(rc == 0, "document_add div succeeds");

    /* List */
    uint64_t list = folio_list_new(helv, 12.0);
    ASSERT(list != 0, "list_new returns handle");

    folio_list_set_style(list, 1); /* ListOrdered */
    folio_list_add_item(list, "First item");
    folio_list_add_item(list, "Second item");
    folio_list_add_item(list, "Third item");

    rc = folio_document_add(doc, list);
    ASSERT(rc == 0, "document_add list succeeds");

    /* Area break */
    uint64_t brk = folio_area_break_new();
    ASSERT(brk != 0, "area_break_new returns handle");
    rc = folio_document_add(doc, brk);
    ASSERT(rc == 0, "document_add area_break succeeds");

    /* Line separator */
    uint64_t sep = folio_line_separator_new();
    ASSERT(sep != 0, "line_separator_new returns handle");
    rc = folio_document_add(doc, sep);
    ASSERT(rc == 0, "document_add line_separator succeeds");

    rc = folio_document_save(doc, "/tmp/folio_cabi_containers.pdf");
    ASSERT(rc == 0, "containers document save succeeds");
    folio_div_free(div);
    folio_list_free(list);
    folio_document_free(doc);

    /* ===== Stage 8: HTML to PDF ===== */
    rc = folio_html_to_pdf("<h1>Hello from C</h1><p>This PDF was generated from HTML via the C ABI.</p>", "/tmp/folio_cabi_html.pdf");
    ASSERT(rc == 0, "html_to_pdf succeeds");

    uint64_t htmlBuf = folio_html_to_buffer("<h1>Buffer Test</h1>", 612, 792);
    ASSERT(htmlBuf != 0, "html_to_buffer returns handle");
    ASSERT(folio_buffer_len(htmlBuf) > 0, "html buffer has data");
    folio_buffer_free(htmlBuf);

    uint64_t htmlDoc = folio_html_convert("<h1>Convert Test</h1><p>Paragraph</p>", 612, 792);
    ASSERT(htmlDoc != 0, "html_convert returns doc handle");
    folio_document_set_title(htmlDoc, "HTML Convert");
    rc = folio_document_save(htmlDoc, "/tmp/folio_cabi_html_convert.pdf");
    ASSERT(rc == 0, "html_convert doc save succeeds");
    folio_document_free(htmlDoc);

    /* ===== Stage 9: Reader ===== */
    /* Read back the HTML PDF we just created */
    uint64_t rdr = folio_reader_open("/tmp/folio_cabi_html.pdf");
    ASSERT(rdr != 0, "reader_open succeeds");

    int32_t pageCount = folio_reader_page_count(rdr);
    ASSERT(pageCount >= 1, "reader page_count >= 1");

    double pw = folio_reader_page_width(rdr, 0);
    ASSERT(pw > 0, "reader page_width > 0");

    double ph = folio_reader_page_height(rdr, 0);
    ASSERT(ph > 0, "reader page_height > 0");

    uint64_t textBuf = folio_reader_extract_text(rdr, 0);
    ASSERT(textBuf != 0, "reader extract_text returns handle");
    ASSERT(folio_buffer_len(textBuf) > 0, "extracted text is non-empty");
    folio_buffer_free(textBuf);

    folio_reader_free(rdr);

    /* ===== Stage 10: Forms & Document Features ===== */
    doc = folio_document_new_letter();
    folio_document_set_title(doc, "Forms Test");
    folio_document_add_page(doc);

    uint64_t form = folio_form_new();
    ASSERT(form != 0, "form_new returns handle");

    rc = folio_form_add_text_field(form, "name", 72, 700, 300, 720, 0);
    ASSERT(rc == 0, "form_add_text_field succeeds");

    rc = folio_form_add_checkbox(form, "agree", 72, 650, 90, 668, 0, 1);
    ASSERT(rc == 0, "form_add_checkbox succeeds");

    rc = folio_document_set_form(doc, form);
    ASSERT(rc == 0, "document_set_form succeeds");

    rc = folio_document_save(doc, "/tmp/folio_cabi_forms.pdf");
    ASSERT(rc == 0, "forms document save succeeds");
    folio_form_free(form);
    folio_document_free(doc);

    /* Document features */
    doc = folio_document_new_letter();
    folio_document_set_title(doc, "Features Test");

    rc = folio_document_set_tagged(doc, 1);
    ASSERT(rc == 0, "set_tagged succeeds");

    rc = folio_document_set_auto_bookmarks(doc, 1);
    ASSERT(rc == 0, "set_auto_bookmarks succeeds");

    folio_document_free(doc);

    /* ===== Stage 11: Callbacks ===== */
    doc = folio_document_new_letter();
    folio_document_set_title(doc, "Callback Test");
    folio_document_add_page(doc);

    /* NULL callback must be rejected */
    rc = folio_document_set_header(doc, NULL, NULL);
    ASSERT(rc != 0, "NULL header callback rejected");

    rc = folio_document_set_footer(doc, NULL, NULL);
    ASSERT(rc != 0, "NULL footer callback rejected");

    folio_document_free(doc);

    /* ===== Stage 12: All 14 standard font accessors ===== */
    printf("Testing all standard font accessors...\n");
    ASSERT(folio_font_helvetica() != 0, "font_helvetica");
    ASSERT(folio_font_helvetica_bold() != 0, "font_helvetica_bold");
    ASSERT(folio_font_helvetica_oblique() != 0, "font_helvetica_oblique");
    ASSERT(folio_font_helvetica_bold_oblique() != 0, "font_helvetica_bold_oblique");
    ASSERT(folio_font_times_roman() != 0, "font_times_roman");
    ASSERT(folio_font_times_bold() != 0, "font_times_bold");
    ASSERT(folio_font_times_italic() != 0, "font_times_italic");
    ASSERT(folio_font_times_bold_italic() != 0, "font_times_bold_italic");
    ASSERT(folio_font_courier() != 0, "font_courier");
    ASSERT(folio_font_courier_bold() != 0, "font_courier_bold");
    ASSERT(folio_font_courier_oblique() != 0, "font_courier_oblique");
    ASSERT(folio_font_courier_bold_oblique() != 0, "font_courier_bold_oblique");
    ASSERT(folio_font_symbol() != 0, "font_symbol");
    ASSERT(folio_font_zapf_dingbats() != 0, "font_zapf_dingbats");

    /* ===== Stage 13: Paragraph extensions ===== */
    printf("Testing paragraph extensions...\n");
    helv = folio_font_helvetica();
    para = folio_paragraph_new("Orphans/widows test paragraph.", helv, 12.0);
    ASSERT(para != 0, "paragraph for extensions");

    rc = folio_paragraph_set_orphans(para, 2);
    ASSERT(rc == 0, "paragraph_set_orphans succeeds");

    rc = folio_paragraph_set_widows(para, 2);
    ASSERT(rc == 0, "paragraph_set_widows succeeds");

    rc = folio_paragraph_set_ellipsis(para, 1);
    ASSERT(rc == 0, "paragraph_set_ellipsis succeeds");

    rc = folio_paragraph_set_word_break(para, "break-all");
    ASSERT(rc == 0, "paragraph_set_word_break succeeds");

    rc = folio_paragraph_set_hyphens(para, "auto");
    ASSERT(rc == 0, "paragraph_set_hyphens succeeds");

    folio_paragraph_free(para);

    /* ===== Stage 14: Table extensions ===== */
    printf("Testing table extensions...\n");
    helv = folio_font_helvetica();

    uint64_t tbl2 = folio_table_new();
    ASSERT(tbl2 != 0, "table for extensions");

    rc = folio_table_set_cell_spacing(tbl2, 2.0, 2.0);
    ASSERT(rc == 0, "table_set_cell_spacing succeeds");

    rc = folio_table_set_auto_column_widths(tbl2);
    ASSERT(rc == 0, "table_set_auto_column_widths succeeds");

    rc = folio_table_set_min_width(tbl2, 200.0);
    ASSERT(rc == 0, "table_set_min_width succeeds");

    /* Footer row */
    uint64_t frow = folio_table_add_footer_row(tbl2);
    ASSERT(frow != 0, "table_add_footer_row returns handle");

    uint64_t fcell = folio_row_add_cell(frow, "Footer", helv, 10.0);
    ASSERT(fcell != 0, "footer cell created");

    /* Header row with cell extensions */
    uint64_t hrow2 = folio_table_add_header_row(tbl2);
    uint64_t hcell = folio_row_add_cell(hrow2, "Header", helv, 12.0);

    rc = folio_cell_set_padding_sides(hcell, 4.0, 8.0, 4.0, 8.0);
    ASSERT(rc == 0, "cell_set_padding_sides succeeds");

    rc = folio_cell_set_valign(hcell, 1); /* VAlignMiddle */
    ASSERT(rc == 0, "cell_set_valign succeeds");

    rc = folio_cell_set_border(hcell, 1.0, 0.0, 0.0, 0.0);
    ASSERT(rc == 0, "cell_set_border succeeds");

    rc = folio_cell_set_width_hint(hcell, 150.0);
    ASSERT(rc == 0, "cell_set_width_hint succeeds");

    /* Cell with element content */
    uint64_t drow2 = folio_table_add_row(tbl2);
    uint64_t cellPara = folio_paragraph_new("Cell content", helv, 10.0);
    uint64_t elemCell = folio_row_add_cell_element(drow2, cellPara);
    ASSERT(elemCell != 0, "row_add_cell_element returns handle");

    folio_table_free(tbl2);

    /* ===== Stage 15: Div extensions ===== */
    printf("Testing div extensions...\n");

    div = folio_div_new();
    rc = folio_div_set_border_radius(div, 8.0);
    ASSERT(rc == 0, "div_set_border_radius succeeds");

    rc = folio_div_set_opacity(div, 0.8);
    ASSERT(rc == 0, "div_set_opacity succeeds");

    rc = folio_div_set_overflow(div, "hidden");
    ASSERT(rc == 0, "div_set_overflow succeeds");

    rc = folio_div_set_max_width(div, 400.0);
    ASSERT(rc == 0, "div_set_max_width succeeds");

    rc = folio_div_set_min_width(div, 100.0);
    ASSERT(rc == 0, "div_set_min_width succeeds");

    rc = folio_div_set_box_shadow(div, 2.0, 2.0, 4.0, 0.0, 0.5, 0.5, 0.5);
    ASSERT(rc == 0, "div_set_box_shadow succeeds");

    rc = folio_div_set_max_height(div, 300.0);
    ASSERT(rc == 0, "div_set_max_height succeeds");

    rc = folio_div_set_space_before(div, 10.0);
    ASSERT(rc == 0, "div_set_space_before succeeds");

    rc = folio_div_set_space_after(div, 10.0);
    ASSERT(rc == 0, "div_set_space_after succeeds");

    folio_div_free(div);

    /* ===== Stage 16: Link element ===== */
    printf("Testing link element...\n");
    helv = folio_font_helvetica();

    uint64_t lnk = folio_link_new("Click here", "https://example.com", helv, 12.0);
    ASSERT(lnk != 0, "link_new returns handle");

    rc = folio_link_set_color(lnk, 0.0, 0.0, 1.0);
    ASSERT(rc == 0, "link_set_color succeeds");

    rc = folio_link_set_underline(lnk);
    ASSERT(rc == 0, "link_set_underline succeeds");

    rc = folio_link_set_align(lnk, 0);
    ASSERT(rc == 0, "link_set_align succeeds");

    folio_link_free(lnk);

    uint64_t intLnk = folio_link_new_internal("Go to chapter", "ch1", helv, 12.0);
    ASSERT(intLnk != 0, "link_new_internal returns handle");
    folio_link_free(intLnk);

    /* ===== Stage 17: Barcode ===== */
    printf("Testing barcode...\n");

    uint64_t qr = folio_barcode_qr("https://folio.dev");
    ASSERT(qr != 0, "barcode_qr returns handle");
    ASSERT(folio_barcode_width(qr) > 0, "barcode_width > 0");
    ASSERT(folio_barcode_height(qr) > 0, "barcode_height > 0");

    uint64_t qrElem = folio_barcode_element_new(qr, 150.0);
    ASSERT(qrElem != 0, "barcode_element_new returns handle");

    rc = folio_barcode_element_set_height(qrElem, 150.0);
    ASSERT(rc == 0, "barcode_element_set_height succeeds");

    rc = folio_barcode_element_set_align(qrElem, 1); /* center */
    ASSERT(rc == 0, "barcode_element_set_align succeeds");

    folio_barcode_element_free(qrElem);
    folio_barcode_free(qr);

    uint64_t qrH = folio_barcode_qr_ecc("test", 3); /* ECC_H */
    ASSERT(qrH != 0, "barcode_qr_ecc returns handle");
    folio_barcode_free(qrH);

    uint64_t c128 = folio_barcode_code128("ABC-123");
    ASSERT(c128 != 0, "barcode_code128 returns handle");
    folio_barcode_free(c128);

    uint64_t ean = folio_barcode_ean13("978020137962");
    ASSERT(ean != 0, "barcode_ean13 returns handle");
    folio_barcode_free(ean);

    /* ===== Stage 18: SVG ===== */
    printf("Testing SVG...\n");

    const char* svgXml = "<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"100\" height=\"100\">"
                         "<circle cx=\"50\" cy=\"50\" r=\"40\" fill=\"red\"/></svg>";
    uint64_t svg = folio_svg_parse(svgXml);
    ASSERT(svg != 0, "svg_parse returns handle");

    double svgW = folio_svg_width(svg);
    ASSERT(svgW > 0, "svg_width > 0");

    double svgH = folio_svg_height(svg);
    ASSERT(svgH > 0, "svg_height > 0");

    uint64_t svgElem = folio_svg_element_new(svg);
    ASSERT(svgElem != 0, "svg_element_new returns handle");

    rc = folio_svg_element_set_size(svgElem, 200.0, 200.0);
    ASSERT(rc == 0, "svg_element_set_size succeeds");

    rc = folio_svg_element_set_align(svgElem, 1);
    ASSERT(rc == 0, "svg_element_set_align succeeds");

    folio_svg_element_free(svgElem);
    folio_svg_free(svg);

    /* svg_parse_bytes */
    uint64_t svgB = folio_svg_parse_bytes(svgXml, (int32_t)strlen(svgXml));
    ASSERT(svgB != 0, "svg_parse_bytes returns handle");
    folio_svg_free(svgB);

    /* ===== Stage 19: Flex container ===== */
    printf("Testing flex container...\n");
    doc = folio_document_new_letter();
    helv = folio_font_helvetica();

    uint64_t flex = folio_flex_new();
    ASSERT(flex != 0, "flex_new returns handle");

    rc = folio_flex_set_direction(flex, 0); /* row */
    ASSERT(rc == 0, "flex_set_direction succeeds");

    rc = folio_flex_set_justify_content(flex, 2); /* center */
    ASSERT(rc == 0, "flex_set_justify_content succeeds");

    rc = folio_flex_set_align_items(flex, 3); /* center */
    ASSERT(rc == 0, "flex_set_align_items succeeds");

    rc = folio_flex_set_wrap(flex, 1); /* wrap */
    ASSERT(rc == 0, "flex_set_wrap succeeds");

    rc = folio_flex_set_gap(flex, 10.0);
    ASSERT(rc == 0, "flex_set_gap succeeds");

    rc = folio_flex_set_row_gap(flex, 8.0);
    ASSERT(rc == 0, "flex_set_row_gap succeeds");

    rc = folio_flex_set_column_gap(flex, 12.0);
    ASSERT(rc == 0, "flex_set_column_gap succeeds");

    rc = folio_flex_set_padding(flex, 10.0);
    ASSERT(rc == 0, "flex_set_padding succeeds");

    rc = folio_flex_set_padding_all(flex, 10.0, 15.0, 10.0, 15.0);
    ASSERT(rc == 0, "flex_set_padding_all succeeds");

    rc = folio_flex_set_background(flex, 0.95, 0.95, 0.95);
    ASSERT(rc == 0, "flex_set_background succeeds");

    rc = folio_flex_set_border(flex, 1.0, 0.0, 0.0, 0.0);
    ASSERT(rc == 0, "flex_set_border succeeds");

    rc = folio_flex_set_space_before(flex, 12.0);
    ASSERT(rc == 0, "flex_set_space_before succeeds");

    rc = folio_flex_set_space_after(flex, 12.0);
    ASSERT(rc == 0, "flex_set_space_after succeeds");

    /* Add elements directly */
    para = folio_paragraph_new("Flex child 1", helv, 12.0);
    rc = folio_flex_add(flex, para);
    ASSERT(rc == 0, "flex_add succeeds");

    /* Add via flex item with properties */
    uint64_t p2 = folio_paragraph_new("Flex child 2", helv, 12.0);
    uint64_t item = folio_flex_item_new(p2);
    ASSERT(item != 0, "flex_item_new returns handle");

    rc = folio_flex_item_set_grow(item, 1.0);
    ASSERT(rc == 0, "flex_item_set_grow succeeds");

    rc = folio_flex_item_set_shrink(item, 0.0);
    ASSERT(rc == 0, "flex_item_set_shrink succeeds");

    rc = folio_flex_item_set_basis(item, 100.0);
    ASSERT(rc == 0, "flex_item_set_basis succeeds");

    rc = folio_flex_item_set_align_self(item, 1); /* start */
    ASSERT(rc == 0, "flex_item_set_align_self succeeds");

    rc = folio_flex_item_set_margins(item, 5.0, 5.0, 5.0, 5.0);
    ASSERT(rc == 0, "flex_item_set_margins succeeds");

    rc = folio_flex_add_item(flex, item);
    ASSERT(rc == 0, "flex_add_item succeeds");

    rc = folio_document_add(doc, flex);
    ASSERT(rc == 0, "document_add flex succeeds");

    rc = folio_document_save(doc, "/tmp/folio_cabi_flex.pdf");
    ASSERT(rc == 0, "flex document save succeeds");

    folio_flex_item_free(item);
    folio_flex_free(flex);
    folio_document_free(doc);

    /* ===== Stage 20: Grid container ===== */
    printf("Testing grid container...\n");
    doc = folio_document_new_letter();
    helv = folio_font_helvetica();

    uint64_t grid = folio_grid_new();
    ASSERT(grid != 0, "grid_new returns handle");

    /* 3 columns: 1fr, 2fr, 1fr */
    int32_t colTypes[] = {2, 2, 2}; /* GridTrackFr */
    double colValues[] = {1.0, 2.0, 1.0};
    rc = folio_grid_set_template_columns(grid, colTypes, colValues, 3);
    ASSERT(rc == 0, "grid_set_template_columns succeeds");

    /* Auto rows with min 50pt */
    int32_t rowTypes[] = {0}; /* GridTrackPx */
    double rowValues[] = {50.0};
    rc = folio_grid_set_auto_rows(grid, rowTypes, rowValues, 1);
    ASSERT(rc == 0, "grid_set_auto_rows succeeds");

    rc = folio_grid_set_gap(grid, 10.0, 10.0);
    ASSERT(rc == 0, "grid_set_gap succeeds");

    rc = folio_grid_set_padding(grid, 10.0);
    ASSERT(rc == 0, "grid_set_padding succeeds");

    rc = folio_grid_set_background(grid, 0.9, 0.95, 1.0);
    ASSERT(rc == 0, "grid_set_background succeeds");

    rc = folio_grid_set_justify_items(grid, 3); /* center */
    ASSERT(rc == 0, "grid_set_justify_items succeeds");

    rc = folio_grid_set_align_items(grid, 3); /* center */
    ASSERT(rc == 0, "grid_set_align_items succeeds");

    rc = folio_grid_set_space_before(grid, 12.0);
    ASSERT(rc == 0, "grid_set_space_before succeeds");

    /* Add children */
    uint64_t gp1 = folio_paragraph_new("Cell A", helv, 12.0);
    rc = folio_grid_add_child(grid, gp1);
    ASSERT(rc == 0, "grid_add_child 1 succeeds");

    uint64_t gp2 = folio_paragraph_new("Cell B (span 2 cols)", helv, 12.0);
    rc = folio_grid_add_child(grid, gp2);
    ASSERT(rc == 0, "grid_add_child 2 succeeds");

    /* Place second child across columns 2-3 */
    rc = folio_grid_set_placement(grid, 1, 2, 4, 0, 0);
    ASSERT(rc == 0, "grid_set_placement succeeds");

    rc = folio_document_add(doc, grid);
    ASSERT(rc == 0, "document_add grid succeeds");

    rc = folio_document_save(doc, "/tmp/folio_cabi_grid.pdf");
    ASSERT(rc == 0, "grid document save succeeds");

    folio_grid_free(grid);
    folio_document_free(doc);

    /* ===== Stage 21: Columns layout ===== */
    printf("Testing columns layout...\n");
    doc = folio_document_new_letter();
    helv = folio_font_helvetica();

    uint64_t cols = folio_columns_new(3);
    ASSERT(cols != 0, "columns_new returns handle");

    rc = folio_columns_set_gap(cols, 20.0);
    ASSERT(rc == 0, "columns_set_gap succeeds");

    double widths[] = {0.25, 0.5, 0.25};
    rc = folio_columns_set_widths(cols, widths, 3);
    ASSERT(rc == 0, "columns_set_widths succeeds");

    uint64_t cp1 = folio_paragraph_new("Left column text.", helv, 10.0);
    rc = folio_columns_add(cols, 0, cp1);
    ASSERT(rc == 0, "columns_add col 0 succeeds");

    uint64_t cp2 = folio_paragraph_new("Center column with more text content.", helv, 10.0);
    rc = folio_columns_add(cols, 1, cp2);
    ASSERT(rc == 0, "columns_add col 1 succeeds");

    uint64_t cp3 = folio_paragraph_new("Right column.", helv, 10.0);
    rc = folio_columns_add(cols, 2, cp3);
    ASSERT(rc == 0, "columns_add col 2 succeeds");

    rc = folio_document_add(doc, cols);
    ASSERT(rc == 0, "document_add columns succeeds");

    rc = folio_document_save(doc, "/tmp/folio_cabi_columns.pdf");
    ASSERT(rc == 0, "columns document save succeeds");

    folio_columns_free(cols);
    folio_document_free(doc);

    /* ===== Stage 22: Float layout ===== */
    printf("Testing float layout...\n");
    doc = folio_document_new_letter();
    helv = folio_font_helvetica();

    uint64_t floatContent = folio_paragraph_new("Floated left box", helv, 10.0);
    uint64_t flt = folio_float_new(0, floatContent); /* FloatLeft */
    ASSERT(flt != 0, "float_new returns handle");

    rc = folio_float_set_margin(flt, 12.0);
    ASSERT(rc == 0, "float_set_margin succeeds");

    rc = folio_document_add(doc, flt);
    ASSERT(rc == 0, "document_add float succeeds");

    rc = folio_document_save(doc, "/tmp/folio_cabi_float.pdf");
    ASSERT(rc == 0, "float document save succeeds");

    folio_float_free(flt);
    folio_document_free(doc);

    /* ===== Stage 23: TabbedLine ===== */
    printf("Testing tabbed line...\n");
    doc = folio_document_new_letter();
    helv = folio_font_helvetica();

    double positions[] = {400.0};
    int32_t aligns[] = {1}; /* TabAlignRight */
    int32_t leaders[] = {'.'}; /* dot leader */

    uint64_t tl = folio_tabbed_line_new(helv, 12.0, positions, aligns, leaders, 1);
    ASSERT(tl != 0, "tabbed_line_new returns handle");

    const char* segments[] = {"Chapter 1", "15"};
    rc = folio_tabbed_line_set_segments(tl, segments, 2);
    ASSERT(rc == 0, "tabbed_line_set_segments succeeds");

    rc = folio_tabbed_line_set_color(tl, 0.0, 0.0, 0.0);
    ASSERT(rc == 0, "tabbed_line_set_color succeeds");

    rc = folio_tabbed_line_set_leading(tl, 1.5);
    ASSERT(rc == 0, "tabbed_line_set_leading succeeds");

    rc = folio_document_add(doc, tl);
    ASSERT(rc == 0, "document_add tabbed_line succeeds");

    rc = folio_document_save(doc, "/tmp/folio_cabi_tabs.pdf");
    ASSERT(rc == 0, "tabbed_line document save succeeds");

    folio_tabbed_line_free(tl);
    folio_document_free(doc);

    /* ===== Stage 24: Watermark & Outlines ===== */
    printf("Testing watermark and outlines...\n");
    doc = folio_document_new_letter();
    helv = folio_font_helvetica();

    rc = folio_document_set_watermark(doc, "DRAFT");
    ASSERT(rc == 0, "document_set_watermark succeeds");

    rc = folio_document_set_watermark_config(doc, "CONFIDENTIAL",
        48.0, 0.8, 0.8, 0.8, 45.0, 0.2);
    ASSERT(rc == 0, "document_set_watermark_config succeeds");

    /* Add content for outlines to reference */
    uint64_t oh1 = folio_heading_new("Chapter 1", 1);
    rc = folio_document_add(doc, oh1);
    ASSERT(rc == 0, "add heading for outline");

    uint64_t outline = folio_document_add_outline(doc, "Chapter 1", 0);
    ASSERT(outline != 0, "document_add_outline returns handle");

    uint64_t child = folio_outline_add_child(outline, "Section 1.1", 0);
    ASSERT(child != 0, "outline_add_child returns handle");

    uint64_t outXyz = folio_document_add_outline_xyz(doc, "Precise", 0, 72.0, 500.0, 1.5);
    ASSERT(outXyz != 0, "document_add_outline_xyz returns handle");

    /* Named destination */
    rc = folio_document_add_named_dest(doc, "ch1", 0, "Fit", 0.0, 0.0, 0.0);
    ASSERT(rc == 0, "document_add_named_dest succeeds");

    /* Viewer preferences */
    rc = folio_document_set_viewer_preferences(doc, "SinglePage", "UseOutlines",
        0, 0, 0, 1, 1, 1);
    ASSERT(rc == 0, "document_set_viewer_preferences succeeds");

    /* Page labels */
    rc = folio_document_add_page_label(doc, 0, "r", "", 1); /* roman numerals */
    ASSERT(rc == 0, "document_add_page_label succeeds");

    rc = folio_document_save(doc, "/tmp/folio_cabi_watermark.pdf");
    ASSERT(rc == 0, "watermark document save succeeds");
    folio_document_free(doc);

    /* ===== Stage 25: Document extensions ===== */
    printf("Testing document extensions...\n");
    doc = folio_document_new_letter();

    /* Page-specific margins */
    rc = folio_document_set_first_margins(doc, 72, 72, 72, 72);
    ASSERT(rc == 0, "document_set_first_margins succeeds");

    rc = folio_document_set_left_margins(doc, 54, 72, 54, 54);
    ASSERT(rc == 0, "document_set_left_margins succeeds");

    rc = folio_document_set_right_margins(doc, 54, 54, 54, 72);
    ASSERT(rc == 0, "document_set_right_margins succeeds");

    /* Inline HTML */
    rc = folio_document_add_html(doc, "<h2>HTML Section</h2><p>Inline HTML content.</p>");
    ASSERT(rc == 0, "document_add_html succeeds");

    rc = folio_document_add_html_with_options(doc,
        "<p>With options</p>", 14.0, 612.0, 792.0, "", "");
    ASSERT(rc == 0, "document_add_html_with_options succeeds");

    /* File attachment */
    const char* xmlData = "<?xml version=\"1.0\"?><invoice><total>100.00</total></invoice>";
    rc = folio_document_attach_file(doc, xmlData, (int32_t)strlen(xmlData),
        "invoice.xml", "application/xml", "Invoice data", "Alternative");
    ASSERT(rc == 0, "document_attach_file succeeds");

    /* Absolute positioning */
    helv = folio_font_helvetica();
    uint64_t absPara = folio_paragraph_new("Absolute positioned", helv, 10.0);
    rc = folio_document_add_absolute(doc, absPara, 100.0, 200.0, 200.0);
    ASSERT(rc == 0, "document_add_absolute succeeds");

    /* Remove page */
    folio_document_add_page(doc);
    count = folio_document_page_count(doc);
    rc = folio_document_remove_page(doc, count - 1);
    ASSERT(rc == 0, "document_remove_page succeeds");

    rc = folio_document_save(doc, "/tmp/folio_cabi_docext.pdf");
    ASSERT(rc == 0, "doc extensions save succeeds");
    folio_document_free(doc);

    /* ===== Stage 26: Page extensions ===== */
    printf("Testing page extensions...\n");
    doc = folio_document_new_letter();
    page = folio_document_add_page(doc);
    helv = folio_font_helvetica();

    rc = folio_page_set_art_box(page, 36, 36, 576, 756);
    ASSERT(rc == 0, "page_set_art_box succeeds");

    rc = folio_page_set_size(page, 612.0, 792.0);
    ASSERT(rc == 0, "page_set_size succeeds");

    rc = folio_page_add_page_link(page, 72, 700, 200, 720, 0);
    ASSERT(rc == 0, "page_add_page_link succeeds");

    rc = folio_page_add_internal_link(page, 72, 670, 200, 690, "ch1");
    ASSERT(rc == 0, "page_add_internal_link succeeds");

    rc = folio_page_add_text_annotation(page, 72, 640, 90, 658, "A note", "Comment");
    ASSERT(rc == 0, "page_add_text_annotation succeeds");

    rc = folio_page_set_opacity_fill_stroke(page, 0.8, 1.0);
    ASSERT(rc == 0, "page_set_opacity_fill_stroke succeeds");

    rc = folio_page_set_crop_box(page, 0, 0, 612, 792);
    ASSERT(rc == 0, "page_set_crop_box succeeds");

    rc = folio_page_set_trim_box(page, 18, 18, 594, 774);
    ASSERT(rc == 0, "page_set_trim_box succeeds");

    rc = folio_page_set_bleed_box(page, 9, 9, 603, 783);
    ASSERT(rc == 0, "page_set_bleed_box succeeds");

    /* Text markup annotations — single quad point */
    double quadPts[] = {72, 600, 200, 600, 200, 612, 72, 612};
    rc = folio_page_add_highlight(page, 72, 600, 200, 612, 1.0, 1.0, 0.0, quadPts, 1);
    ASSERT(rc == 0, "page_add_highlight succeeds");

    rc = folio_page_add_underline_annotation(page, 72, 580, 200, 592, 0.0, 0.0, 1.0, quadPts, 1);
    ASSERT(rc == 0, "page_add_underline_annotation succeeds");

    rc = folio_page_add_squiggly(page, 72, 560, 200, 572, 1.0, 0.0, 0.0, quadPts, 1);
    ASSERT(rc == 0, "page_add_squiggly succeeds");

    rc = folio_page_add_strikeout(page, 72, 540, 200, 552, 0.5, 0.0, 0.0, quadPts, 1);
    ASSERT(rc == 0, "page_add_strikeout succeeds");

    rc = folio_document_save(doc, "/tmp/folio_cabi_pageext.pdf");
    ASSERT(rc == 0, "page extensions save succeeds");
    folio_document_free(doc);

    /* ===== Stage 27: Forms extensions ===== */
    printf("Testing forms extensions...\n");
    doc = folio_document_new_letter();
    folio_document_add_page(doc);

    form = folio_form_new();

    /* Additional field types */
    rc = folio_form_add_multiline_text_field(form, "notes", 72, 600, 300, 700, 0);
    ASSERT(rc == 0, "form_add_multiline_text_field succeeds");

    rc = folio_form_add_password_field(form, "pwd", 72, 560, 300, 580, 0);
    ASSERT(rc == 0, "form_add_password_field succeeds");

    const char* listOpts[] = {"Option A", "Option B", "Option C"};
    rc = folio_form_add_listbox(form, "choices", 72, 400, 200, 540, 0, listOpts, 3);
    ASSERT(rc == 0, "form_add_listbox succeeds");

    /* Radio group */
    const char* radioVals[] = {"yes", "no"};
    double radioRects[] = {72, 350, 90, 368,  120, 350, 138, 368};
    int32_t radioPages[] = {0, 0};
    rc = folio_form_add_radio_group(form, "confirm", radioVals, radioRects, radioPages, 2);
    ASSERT(rc == 0, "form_add_radio_group succeeds");

    /* Field builder pattern */
    uint64_t field = folio_form_create_text_field("email", 72, 300, 300, 320, 0);
    ASSERT(field != 0, "form_create_text_field returns handle");

    rc = folio_form_field_set_value(field, "user@example.com");
    ASSERT(rc == 0, "form_field_set_value succeeds");

    rc = folio_form_field_set_required(field);
    ASSERT(rc == 0, "form_field_set_required succeeds");

    rc = folio_form_field_set_background_color(field, 1.0, 1.0, 0.9);
    ASSERT(rc == 0, "form_field_set_background_color succeeds");

    rc = folio_form_field_set_border_color(field, 0.0, 0.0, 0.5);
    ASSERT(rc == 0, "form_field_set_border_color succeeds");

    rc = folio_form_add_field(form, field);
    ASSERT(rc == 0, "form_add_field succeeds");

    /* Read-only checkbox */
    uint64_t roCheck = folio_form_create_checkbox("locked", 72, 260, 90, 278, 0, 1);
    ASSERT(roCheck != 0, "form_create_checkbox returns handle");
    rc = folio_form_field_set_read_only(roCheck);
    ASSERT(rc == 0, "form_field_set_read_only succeeds");
    rc = folio_form_add_field(form, roCheck);
    ASSERT(rc == 0, "form_add_field (read-only checkbox) succeeds");

    rc = folio_document_set_form(doc, form);
    ASSERT(rc == 0, "document_set_form with extensions succeeds");

    rc = folio_document_save(doc, "/tmp/folio_cabi_forms_ext.pdf");
    ASSERT(rc == 0, "forms extension save succeeds");

    folio_form_field_free(field);
    folio_form_field_free(roCheck);
    folio_form_free(form);
    folio_document_free(doc);

    /* ===== Stage 28: Form filling ===== */
    printf("Testing form filling...\n");
    /* Re-open the forms PDF we just saved */
    uint64_t fillRdr = folio_reader_open("/tmp/folio_cabi_forms.pdf");
    ASSERT(fillRdr != 0, "reader_open for form filling");

    uint64_t filler = folio_form_filler_new(fillRdr);
    ASSERT(filler != 0, "form_filler_new returns handle");

    uint64_t namesBuf = folio_form_filler_field_names(filler);
    ASSERT(namesBuf != 0, "form_filler_field_names returns buffer");
    ASSERT(folio_buffer_len(namesBuf) > 0, "field names non-empty");
    folio_buffer_free(namesBuf);

    rc = folio_form_filler_set_value(filler, "name", "John Doe");
    ASSERT(rc == 0, "form_filler_set_value succeeds");

    uint64_t valBuf = folio_form_filler_get_value(filler, "name");
    ASSERT(valBuf != 0, "form_filler_get_value returns buffer");
    folio_buffer_free(valBuf);

    rc = folio_form_filler_set_checkbox(filler, "agree", 0);
    ASSERT(rc == 0, "form_filler_set_checkbox succeeds");

    folio_form_filler_free(filler);
    folio_reader_free(fillRdr);

    /* ===== Stage 29: Image element extensions ===== */
    printf("Testing image element align...\n");
    /* We can't test actual image loading without a file, but we test
       that the set_align function exists and handles bad handles */
    rc = folio_image_element_set_align(99999, 1);
    ASSERT(rc != 0, "image_element_set_align rejects bad handle");

    /* ===== Stage 30: List extensions ===== */
    printf("Testing list extensions...\n");
    helv = folio_font_helvetica();
    list = folio_list_new(helv, 12.0);

    rc = folio_list_set_leading(list, 1.5);
    ASSERT(rc == 0, "list_set_leading succeeds");

    folio_list_add_item(list, "Parent item");
    uint64_t subList = folio_list_add_nested_item(list, "Nested parent");
    ASSERT(subList != 0, "list_add_nested_item returns sub-list handle");

    folio_list_add_item(subList, "Sub-item A");
    folio_list_add_item(subList, "Sub-item B");

    folio_list_free(list);

    /* ===== Stage 31: PDF Merge ===== */
    printf("Testing PDF merge...\n");

    /* Create two simple PDFs to merge */
    doc = folio_document_new_letter();
    helv = folio_font_helvetica();
    para = folio_paragraph_new("Page from doc 1", helv, 14.0);
    folio_document_add(doc, para);
    folio_document_save(doc, "/tmp/folio_merge_a.pdf");
    folio_document_free(doc);

    doc = folio_document_new_a4();
    para = folio_paragraph_new("Page from doc 2", helv, 14.0);
    folio_document_add(doc, para);
    folio_document_save(doc, "/tmp/folio_merge_b.pdf");
    folio_document_free(doc);

    /* Merge via file paths */
    const char* mergePaths[] = {"/tmp/folio_merge_a.pdf", "/tmp/folio_merge_b.pdf"};
    uint64_t merged = folio_merge_files((char**)mergePaths, 2);
    ASSERT(merged != 0, "merge_files returns handle");

    rc = folio_merge_set_info(merged, "Merged PDF", "Test");
    ASSERT(rc == 0, "merge_set_info succeeds");

    rc = folio_merge_add_blank_page(merged, 612, 792);
    ASSERT(rc == 0, "merge_add_blank_page succeeds");

    rc = folio_merge_add_page_with_text(merged, 612, 792,
        "Added page", helv, 12.0, 72.0, 700.0);
    ASSERT(rc == 0, "merge_add_page_with_text succeeds");

    rc = folio_merge_save(merged, "/tmp/folio_merged.pdf");
    ASSERT(rc == 0, "merge_save succeeds");

    folio_merge_free(merged);

    /* Merge via reader handles */
    uint64_t r1 = folio_reader_open("/tmp/folio_merge_a.pdf");
    uint64_t r2 = folio_reader_open("/tmp/folio_merge_b.pdf");
    ASSERT(r1 != 0 && r2 != 0, "readers for merge opened");

    uint64_t readerHandles[] = {r1, r2};
    merged = folio_reader_merge(readerHandles, 2);
    ASSERT(merged != 0, "reader_merge returns handle");

    uint64_t mergeBuf = folio_merge_write_to_buffer(merged);
    ASSERT(mergeBuf != 0, "merge_write_to_buffer returns handle");
    ASSERT(folio_buffer_len(mergeBuf) > 0, "merged buffer has data");
    folio_buffer_free(mergeBuf);

    folio_merge_free(merged);
    folio_reader_free(r1);
    folio_reader_free(r2);

    /* ===== Stage 32: TextRun builder ===== */
    printf("Testing TextRun builder...\n");
    helv = folio_font_helvetica();

    uint64_t rl = folio_run_list_new();
    ASSERT(rl != 0, "run_list_new returns handle");

    rc = folio_run_list_add(rl, "Normal text. ", helv, 12.0, 0.0, 0.0, 0.0);
    ASSERT(rc == 0, "run_list_add succeeds");

    rc = folio_run_list_add(rl, "Bold red. ", folio_font_helvetica_bold(), 12.0, 1.0, 0.0, 0.0);
    ASSERT(rc == 0, "run_list_add bold succeeds");

    rc = folio_run_list_add_link(rl, "Click here", helv, 12.0,
        0.0, 0.0, 1.0, "https://example.com", 1);
    ASSERT(rc == 0, "run_list_add_link succeeds");

    rc = folio_run_list_last_set_letter_spacing(rl, 0.5);
    ASSERT(rc == 0, "run_list_last_set_letter_spacing succeeds");

    /* Apply runs to a heading */
    uint64_t runH = folio_heading_new("placeholder", 2);
    rc = folio_heading_set_runs(runH, rl);
    ASSERT(rc == 0, "heading_set_runs succeeds");

    /* Apply runs to a list item */
    list = folio_list_new(helv, 12.0);
    uint64_t rl2 = folio_run_list_new();
    folio_run_list_add(rl2, "Styled item", helv, 12.0, 0.0, 0.0, 0.0);
    folio_run_list_last_set_underline(rl2);

    rc = folio_list_add_item_runs(list, rl2);
    ASSERT(rc == 0, "list_add_item_runs succeeds");

    uint64_t rl3 = folio_run_list_new();
    folio_run_list_add(rl3, "Parent with sub", helv, 12.0, 0.0, 0.0, 0.0);
    uint64_t subFromRuns = folio_list_add_item_runs_with_sublist(list, rl3);
    ASSERT(subFromRuns != 0, "list_add_item_runs_with_sublist returns sub-list");

    /* Build a doc with the styled heading and list */
    doc = folio_document_new_letter();
    folio_document_add(doc, runH);
    folio_document_add(doc, list);
    rc = folio_document_save(doc, "/tmp/folio_cabi_textrun.pdf");
    ASSERT(rc == 0, "textrun document save succeeds");

    folio_run_list_free(rl);
    folio_run_list_free(rl2);
    folio_run_list_free(rl3);
    folio_heading_free(runH);
    folio_list_free(list);
    folio_document_free(doc);

    /* ===== Stage 33: Digital Signatures ===== */
    printf("Testing digital signatures...\n");

    /* Embedded test key and cert (self-signed RSA 2048, CN=Folio Test) */
    static const char test_key_pem[] =
        "-----BEGIN PRIVATE KEY-----\n"
        "MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCNHIB8Wg853Ty1\n"
        "TpY3vizEquniPCvZo4+Tu8z0RwsUcADbI/n1v8RVA20GkXZTw1odQ4S6hrjU05Z3\n"
        "T1AGUH+Gii+x5G1Zikio/MZ3FH89VxXeQdHcZsfskyiCCW1Xr8YQ0u0YbRt60OtL\n"
        "co+UM8uX/pR4yxuWFAlMvei2AV3a/12S/Z8130HpA6nIKtlNsF1jSwAlPz5uz2w8\n"
        "idtNk+40Rg6KdFOwaGI4sn4xQsLXuNFuvgrctOJQlXvy7qn+hFXWzeL85aPznlhD\n"
        "uY0kfpDiypvy5hPwL1bm12159V8ORaYqjmYXM2Uwe5lwo0/qoA5UghcpNE1C2XUq\n"
        "vKR5Bh4TAgMBAAECggEALdapYju7IHfepaLep0LutgAzM5uKPwujs0DZRizhy4TK\n"
        "smUQaKUfuOIBA5YzEuitmD37mr/ywKfvA9jOzQ5PLrwq3CWw49pI2iQqsDoHTzy/\n"
        "zNAixO7aWN5A1WUMkOR3ebWa8UlYGOBO3tZbD7B3nWtEfay0zF672qMCOkVdYKsC\n"
        "ELR9yo/EcGimVONocd4Ng7AcuEqkw1vQX8PRFGcWIU2hilZ0QWrILczxpU4UZ10W\n"
        "luoGFDMKilVwl80pR0cNipBTXFpCSFd64yGEq64oAwAKsUyu+U8x8JqY4NfKmDKd\n"
        "Fl1o0l3gCiV/U01C1sbg6UA1N57ydNi+C2BX4B+wYQKBgQDBl9tupP5jgIoKHPlP\n"
        "PgGAKYYnmJojvmxnOL981L555e2Aa7izpvMJgtTLao41C1I/yGoSzqbuAyEzbuXY\n"
        "DoMfxiM8iYGQVPsMDmyfHnsh8QJ4mezvJtUDH5AVlmDF4rlNVUM2J4fBudYKK9Ba\n"
        "on4DGcNdSnyqCSLb3adiLO4MsQKBgQC6mZ3AphJdFtkwfcET3USw4XAWv/OBq8Y3\n"
        "iW8W1iVOTWks8kz2zH3BwPOaPgItv+BxWb7Y3xVdZX4PenQYEvIDh1fDSmDVk1ZV\n"
        "wr7ecuCyZxvlVKCvQR6k7Owm6v3obUxh9d0JPhO40dBQmNXqIleCvomYEd6iOtdb\n"
        "MMuszxl4AwKBgDdPT54Y/aUoAEhLZt9A8+TN+EovyzoTGZ2UFQFxNspU6roaDLcn\n"
        "J6C8BFfppZ96EJGwjL261O6DbmTndjt4SfY2AbM2BhJadS9In7gGWV+lH/Oc5pid\n"
        "G5a82cmRISzoTdDOX1mFib/LfF+DjQ+YBTCVl797nIpn+dGfTkc6VvjBAoGAZdFM\n"
        "csIrYwAEoqD7qYsi4CEiuEck56yDC+rB4wQWdxNmQFA2DrSmKgLO6WrMZYo9PZxq\n"
        "touDbOLkVi3LCxfLVoGTMcxz6hXrFgCdhWPJl48Leacjxn0R64wbj2DKCvkYpJsz\n"
        "cMeem0phEJlUn9z9QyoUihW2FW9l8yw89p5Kl2ECgYEAlx6vE0trutkMTPqBPF0T\n"
        "GsEWTMl4T/rBjg44uhqnEeCbmP+c/AdoRvIDFsmL3HeGVpXdPY2ErXxNGTgVDiHf\n"
        "7SLFmOzww1Fb8l3qtO08/aUTgDY2oYUIm3HxjgwKhhDGu0HkQKjHMLiDUs2kbHBO\n"
        "VhUNHIavjT9GTx1xkW9wzVg=\n"
        "-----END PRIVATE KEY-----\n";

    static const char test_cert_pem[] =
        "-----BEGIN CERTIFICATE-----\n"
        "MIIDKTCCAhGgAwIBAgIUO1af5HyjPhg326gfYL84I/tEMbwwDQYJKoZIhvcNAQEL\n"
        "BQAwJDETMBEGA1UEAwwKRm9saW8gVGVzdDENMAsGA1UECgwEVGVzdDAeFw0yNjAz\n"
        "MjUwNjI1NDBaFw0yNzAzMjUwNjI1NDBaMCQxEzARBgNVBAMMCkZvbGlvIFRlc3Qx\n"
        "DTALBgNVBAoMBFRlc3QwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQCN\n"
        "HIB8Wg853Ty1TpY3vizEquniPCvZo4+Tu8z0RwsUcADbI/n1v8RVA20GkXZTw1od\n"
        "Q4S6hrjU05Z3T1AGUH+Gii+x5G1Zikio/MZ3FH89VxXeQdHcZsfskyiCCW1Xr8YQ\n"
        "0u0YbRt60OtLco+UM8uX/pR4yxuWFAlMvei2AV3a/12S/Z8130HpA6nIKtlNsF1j\n"
        "SwAlPz5uz2w8idtNk+40Rg6KdFOwaGI4sn4xQsLXuNFuvgrctOJQlXvy7qn+hFXW\n"
        "zeL85aPznlhDuY0kfpDiypvy5hPwL1bm12159V8ORaYqjmYXM2Uwe5lwo0/qoA5U\n"
        "ghcpNE1C2XUqvKR5Bh4TAgMBAAGjUzBRMB0GA1UdDgQWBBSqc6CkXSGCgZANDoiW\n"
        "xkMQVoiNEzAfBgNVHSMEGDAWgBSqc6CkXSGCgZANDoiWxkMQVoiNEzAPBgNVHRMB\n"
        "Af8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQANCJL0Rw97pLM8r9H0N8YlaBtJ\n"
        "sp4sE6xHFbrZ05U6AlJdN2EAYpzBW7CkBF1o4lCZ3Aj28NoCkaL6Rfspr5yx8FKC\n"
        "panfFP7M7HsZlrZxhyCCxiDid+xcgI0YEpwtwg8LebzmS3pYcstopD3/e9dDDeCe\n"
        "TvJWAbhAOGvUyl5+Lv/yWPGAB2UyaeX1gXlbhJgdEF5MrJ/jsiYlX3yaBpZB81tw\n"
        "6KsMBoKtX+cJ7q7vy4/kzGQyvWuX+AtGRuK2OT5EHpuyonPMkv+kiAlC6tEumVfN\n"
        "2k3yGjRXibzFKshYrFQquqkrjb5WEcN5CjZOwZKEx9S8TjZWw5rjaff3CmMA\n"
        "-----END CERTIFICATE-----\n";

    /* Test invalid PEM */
    uint64_t badSigner = folio_signer_new_pem("bad", 3, "bad", 3);
    ASSERT(badSigner == 0, "signer_new_pem rejects invalid PEM");

    /* Create signer from test PEM */
    uint64_t signer = folio_signer_new_pem(
        test_key_pem, (int32_t)strlen(test_key_pem),
        test_cert_pem, (int32_t)strlen(test_cert_pem));
    ASSERT(signer != 0, "signer_new_pem succeeds with test PEM");

    /* TSA and OCSP clients */
    uint64_t tsa = folio_tsa_client_new("http://timestamp.digicert.com");
    ASSERT(tsa != 0, "tsa_client_new returns handle");

    uint64_t ocsp = folio_ocsp_client_new();
    ASSERT(ocsp != 0, "ocsp_client_new returns handle");

    /* Build sign options */
    uint64_t signOpts = folio_sign_opts_new(signer, 0 /* BB */);
    ASSERT(signOpts != 0, "sign_opts_new returns handle");

    rc = folio_sign_opts_set_name(signOpts, "Test Signer");
    ASSERT(rc == 0, "sign_opts_set_name succeeds");

    rc = folio_sign_opts_set_reason(signOpts, "Testing");
    ASSERT(rc == 0, "sign_opts_set_reason succeeds");

    rc = folio_sign_opts_set_location(signOpts, "Test Lab");
    ASSERT(rc == 0, "sign_opts_set_location succeeds");

    rc = folio_sign_opts_set_contact_info(signOpts, "test@example.com");
    ASSERT(rc == 0, "sign_opts_set_contact_info succeeds");

    /* Create a PDF to sign */
    doc = folio_document_new_letter();
    helv = folio_font_helvetica();
    para = folio_paragraph_new("Document to be signed", helv, 14.0);
    folio_document_add(doc, para);
    buf = folio_document_write_to_buffer(doc);
    ASSERT(buf != 0, "PDF for signing created");
    folio_document_free(doc);

    /* Sign the PDF (B-B level, no TSA needed) */
    void* pdfData = folio_buffer_data(buf);
    int32_t pdfLen = folio_buffer_len(buf);
    uint64_t signedBuf = folio_sign_pdf(pdfData, pdfLen, signOpts);
    ASSERT(signedBuf != 0, "sign_pdf returns buffer handle");
    ASSERT(folio_buffer_len(signedBuf) > pdfLen, "signed PDF is larger than original");

    void* signedData = folio_buffer_data(signedBuf);
    ASSERT(memcmp(signedData, "%PDF", 4) == 0, "signed output is valid PDF");

    /* Verify signed PDF contains signature markers */
    char* signedStr = (char*)signedData;
    int32_t signedLen2 = folio_buffer_len(signedBuf);
    int hasSigField = 0;
    for (int32_t i = 0; i < signedLen2 - 4; i++) {
        if (signedStr[i] == '/' && signedStr[i+1] == 'S' && signedStr[i+2] == 'i' && signedStr[i+3] == 'g') {
            hasSigField = 1;
            break;
        }
    }
    ASSERT(hasSigField, "signed PDF contains /Sig field");

    folio_buffer_free(signedBuf);
    folio_buffer_free(buf);
    folio_sign_opts_free(signOpts);
    folio_tsa_client_free(tsa);
    folio_ocsp_client_free(ocsp);
    folio_signer_free(signer);

    /* ===== Stage 34: Page manipulation ===== */
    printf("Testing page manipulation...\n");

    /* Create a 3-page merged document */
    doc = folio_document_new_letter();
    para = folio_paragraph_new("Page 1", helv, 14.0);
    folio_document_add(doc, para);
    folio_document_save(doc, "/tmp/folio_manip_1.pdf");
    folio_document_free(doc);

    doc = folio_document_new_letter();
    para = folio_paragraph_new("Page 2", helv, 14.0);
    folio_document_add(doc, para);
    folio_document_save(doc, "/tmp/folio_manip_2.pdf");
    folio_document_free(doc);

    doc = folio_document_new_letter();
    para = folio_paragraph_new("Page 3", helv, 14.0);
    folio_document_add(doc, para);
    folio_document_save(doc, "/tmp/folio_manip_3.pdf");
    folio_document_free(doc);

    const char* manipPaths[] = {"/tmp/folio_manip_1.pdf", "/tmp/folio_manip_2.pdf", "/tmp/folio_manip_3.pdf"};
    merged = folio_merge_files((char**)manipPaths, 3);
    ASSERT(merged != 0, "merge 3 files for manipulation");

    int32_t pc = folio_merge_page_count(merged);
    ASSERT(pc == 3, "merge_page_count is 3");

    /* Rotate page 0 */
    rc = folio_merge_rotate_page(merged, 0, 90);
    ASSERT(rc == 0, "merge_rotate_page succeeds");

    /* Crop page 1 */
    rc = folio_merge_crop_page(merged, 1, 36, 36, 576, 756);
    ASSERT(rc == 0, "merge_crop_page succeeds");

    /* Remove page 2 */
    rc = folio_merge_remove_page(merged, 2);
    ASSERT(rc == 0, "merge_remove_page succeeds");

    pc = folio_merge_page_count(merged);
    ASSERT(pc == 2, "page count is 2 after remove");

    /* Reorder: reverse */
    int32_t reverseOrder[] = {1, 0};
    rc = folio_merge_reorder_pages(merged, reverseOrder, 2);
    ASSERT(rc == 0, "merge_reorder_pages succeeds");

    rc = folio_merge_save(merged, "/tmp/folio_manipulated.pdf");
    ASSERT(rc == 0, "manipulated document save succeeds");

    /* Verify manipulated PDF via write-to-buffer */
    uint64_t manipBuf = folio_merge_write_to_buffer(merged);
    ASSERT(manipBuf != 0, "manipulated write_to_buffer");
    ASSERT(memcmp(folio_buffer_data(manipBuf), "%PDF", 4) == 0, "manipulated output is PDF");
    /* Check /Rotate is present in the output (page 0 was rotated 90) */
    char* manipStr = (char*)folio_buffer_data(manipBuf);
    int32_t manipLen2 = folio_buffer_len(manipBuf);
    int hasRotate = 0;
    for (int32_t i = 0; i < manipLen2 - 7; i++) {
        if (manipStr[i] == '/' && manipStr[i+1] == 'R' && manipStr[i+2] == 'o' &&
            manipStr[i+3] == 't' && manipStr[i+4] == 'a' && manipStr[i+5] == 't' && manipStr[i+6] == 'e') {
            hasRotate = 1;
            break;
        }
    }
    ASSERT(hasRotate, "manipulated PDF contains /Rotate");
    folio_buffer_free(manipBuf);
    folio_merge_free(merged);

    /* ===== Stage 35: Structured content extraction ===== */
    printf("Testing structured content extraction...\n");

    /* Create a simple PDF with text */
    doc = folio_document_new_letter();
    helv = folio_font_helvetica();
    para = folio_paragraph_new("Extractable text content", helv, 14.0);
    folio_document_add(doc, para);
    buf = folio_document_write_to_buffer(doc);
    folio_document_free(doc);

    /* Parse it back */
    uint64_t extRdr = folio_reader_parse(folio_buffer_data(buf), folio_buffer_len(buf));
    ASSERT(extRdr != 0, "reader_parse for extraction");

    /* Text spans */
    uint64_t spansBuf = folio_reader_text_spans(extRdr, 0);
    ASSERT(spansBuf != 0, "reader_text_spans returns buffer");
    ASSERT(folio_buffer_len(spansBuf) > 2, "text spans JSON is non-empty");
    /* Verify JSON array start */
    char* spansJson = (char*)folio_buffer_data(spansBuf);
    ASSERT(spansJson[0] == '[', "text spans JSON starts with [");
    folio_buffer_free(spansBuf);

    /* Images (may be empty array) */
    uint64_t imgBuf = folio_reader_images(extRdr, 0);
    ASSERT(imgBuf != 0, "reader_images returns buffer");
    folio_buffer_free(imgBuf);

    /* Paths */
    uint64_t pathBuf = folio_reader_paths(extRdr, 0);
    ASSERT(pathBuf != 0, "reader_paths returns buffer");
    folio_buffer_free(pathBuf);

    folio_reader_free(extRdr);
    folio_buffer_free(buf);

    /* ===== Stage 36: Form flattening ===== */
    printf("Testing form flattening...\n");

    /* Open the forms PDF we created earlier */
    uint64_t flatRdr = folio_reader_open("/tmp/folio_cabi_forms.pdf");
    ASSERT(flatRdr != 0, "reader_open for flattening");

    uint64_t flatReaders[] = {flatRdr};
    uint64_t flatMerged = folio_reader_merge(flatReaders, 1);
    ASSERT(flatMerged != 0, "merge for flattening");

    rc = folio_merge_flatten_forms(flatMerged);
    ASSERT(rc == 0, "merge_flatten_forms succeeds");

    rc = folio_merge_save(flatMerged, "/tmp/folio_flattened.pdf");
    ASSERT(rc == 0, "flattened document save succeeds");

    /* Verify the flattened PDF has no AcroForm */
    uint64_t flatBuf = folio_merge_write_to_buffer(flatMerged);
    ASSERT(flatBuf != 0, "flattened write_to_buffer");
    char* flatStr = (char*)folio_buffer_data(flatBuf);
    int32_t flatLen2 = folio_buffer_len(flatBuf);
    int hasAcroForm = 0;
    for (int32_t i = 0; i < flatLen2 - 9; i++) {
        if (flatStr[i] == '/' && flatStr[i+1] == 'A' && flatStr[i+2] == 'c' &&
            flatStr[i+3] == 'r' && flatStr[i+4] == 'o' && flatStr[i+5] == 'F') {
            hasAcroForm = 1;
            break;
        }
    }
    ASSERT(!hasAcroForm, "flattened PDF has no /AcroForm");
    folio_buffer_free(flatBuf);

    uint64_t flatCheck = folio_reader_open("/tmp/folio_flattened.pdf");
    ASSERT(flatCheck != 0, "re-open flattened PDF");
    ASSERT(folio_reader_page_count(flatCheck) >= 1, "flattened PDF has pages");
    folio_reader_free(flatCheck);

    folio_merge_free(flatMerged);
    folio_reader_free(flatRdr);

    /* ===== Stage 37: PDF/UA tagged PDF ===== */
    printf("Testing PDF/UA tagged PDF...\n");
    doc = folio_document_new_letter();
    helv = folio_font_helvetica();

    rc = folio_document_set_tagged(doc, 1);
    ASSERT(rc == 0, "set_tagged succeeds");

    /* Image with alt text */
    uint64_t tagQr = folio_barcode_qr("https://folio.dev");
    uint64_t tagQrElem = folio_barcode_element_new(tagQr, 100.0);
    rc = folio_barcode_element_set_alt_text(tagQrElem, "QR code linking to folio.dev");
    ASSERT(rc == 0, "barcode_element_set_alt_text succeeds");
    folio_document_add(doc, tagQrElem);

    /* Div with custom tag */
    div = folio_div_new();
    rc = folio_div_set_tag(div, "Sect");
    ASSERT(rc == 0, "div_set_tag succeeds");
    para = folio_paragraph_new("Section content", helv, 12.0);
    folio_div_add(div, para);
    folio_document_add(doc, div);

    buf = folio_document_write_to_buffer(doc);
    ASSERT(buf != 0, "tagged doc write to buffer");
    folio_document_free(doc);

    /* Read back and check structure tree */
    uint64_t tagRdr = folio_reader_parse(folio_buffer_data(buf), folio_buffer_len(buf));
    ASSERT(tagRdr != 0, "reader_parse tagged doc");

    uint64_t treeBuf = folio_reader_structure_tree(tagRdr);
    ASSERT(treeBuf != 0, "reader_structure_tree returns buffer");
    ASSERT(folio_buffer_len(treeBuf) > 10, "structure tree JSON non-trivial");
    char* treeJson = (char*)folio_buffer_data(treeBuf);
    ASSERT(treeJson[0] == '{', "structure tree JSON starts with {");
    folio_buffer_free(treeBuf);

    folio_reader_free(tagRdr);
    folio_buffer_free(buf);

    /* ===== Stage 38: Encryption with permissions ===== */
    printf("Testing encryption with permissions...\n");
    doc = folio_document_new_letter();
    folio_document_set_title(doc, "Encrypted");
    helv = folio_font_helvetica();
    para = folio_paragraph_new("Encrypted content", helv, 12.0);
    folio_document_add(doc, para);

    rc = folio_document_set_encryption_with_permissions(doc,
        "user123", "owner456", 2 /* AES-256 */,
        (1 << 2) | (1 << 4) /* PRINT | EXTRACT */);
    ASSERT(rc == 0, "set_encryption_with_permissions succeeds");

    buf = folio_document_write_to_buffer(doc);
    ASSERT(buf != 0, "encrypted doc write to buffer");
    ASSERT(memcmp(folio_buffer_data(buf), "%PDF", 4) == 0, "encrypted output is PDF");
    /* Verify /Encrypt dict present */
    char* encStr = (char*)folio_buffer_data(buf);
    int32_t encLen = folio_buffer_len(buf);
    int hasEncrypt = 0;
    for (int32_t i = 0; i < encLen - 8; i++) {
        if (encStr[i] == '/' && encStr[i+1] == 'E' && encStr[i+2] == 'n' &&
            encStr[i+3] == 'c' && encStr[i+4] == 'r' && encStr[i+5] == 'y' &&
            encStr[i+6] == 'p' && encStr[i+7] == 't') {
            hasEncrypt = 1;
            break;
        }
    }
    ASSERT(hasEncrypt, "encrypted PDF contains /Encrypt dictionary");
    folio_buffer_free(buf);
    folio_document_free(doc);

    /* ===== Stage 39: v0.6.2 exports ===== */
    printf("Testing v0.6.2 exports (ToBytes, Div/Cell/Grid/Flex extras, image fit, highlights, PKCS12, CSS length)...\n");

    /* Document.ToBytes — convenience */
    doc = folio_document_new_letter();
    helv = folio_font_helvetica();
    para = folio_paragraph_new("ToBytes test", helv, 14.0);
    folio_document_add(doc, para);
    uint64_t tbBuf = folio_document_to_bytes(doc);
    ASSERT(tbBuf != 0, "document_to_bytes returns buffer");
    ASSERT(folio_buffer_len(tbBuf) > 100, "to_bytes buffer has content");
    ASSERT(memcmp(folio_buffer_data(tbBuf), "%PDF", 4) == 0, "to_bytes output is PDF");
    folio_buffer_free(tbBuf);
    folio_document_free(doc);

    /* Document.ValidatePdfA — returns OK when PDF/A not configured (no-op) */
    doc = folio_document_new_letter();
    rc = folio_document_validate_pdfa(doc);
    ASSERT(rc == 0, "validate_pdfa no-op when PDF/A not configured");
    folio_document_free(doc);

    /* Div extensions */
    doc = folio_document_new_letter();
    div = folio_div_new();
    rc = folio_div_set_aspect_ratio(div, 16.0/9.0);
    ASSERT(rc == 0, "div_set_aspect_ratio succeeds");
    rc = folio_div_set_keep_together(div, 1);
    ASSERT(rc == 0, "div_set_keep_together succeeds");
    rc = folio_div_set_border_radius_per_corner(div, 4, 8, 4, 8);
    ASSERT(rc == 0, "div_set_border_radius_per_corner succeeds");
    rc = folio_div_set_width_percent(div, 0.5);
    ASSERT(rc == 0, "div_set_width_percent succeeds");
    rc = folio_div_set_hcenter(div, 1);
    ASSERT(rc == 0, "div_set_hcenter succeeds");
    rc = folio_div_set_outline(div, 2.0, "solid", 0.2, 0.2, 0.2, 2.0);
    ASSERT(rc == 0, "div_set_outline succeeds");
    rc = folio_div_add_box_shadow(div, 2, 2, 4, 0, 0.5, 0.5, 0.5);
    ASSERT(rc == 0, "div_add_box_shadow succeeds");
    rc = folio_div_set_clear(div, "both");
    ASSERT(rc == 0, "div_set_clear succeeds");
    rc = folio_div_set_hright(div, 0);
    ASSERT(rc == 0, "div_set_hright succeeds");

    para = folio_paragraph_new("Styled div content", helv, 12.0);
    folio_div_add(div, para);
    folio_document_add(doc, div);

    /* Cell border radius */
    uint64_t tbl62 = folio_table_new();
    uint64_t hrow62 = folio_table_add_header_row(tbl62);
    uint64_t hcell62 = folio_row_add_cell(hrow62, "Header", helv, 12.0);
    rc = folio_cell_set_border_radius(hcell62, 4.0);
    ASSERT(rc == 0, "cell_set_border_radius succeeds");
    rc = folio_cell_set_border_radius_per_corner(hcell62, 6, 6, 0, 0);
    ASSERT(rc == 0, "cell_set_border_radius_per_corner succeeds");
    folio_document_add(doc, tbl62);

    /* Grid extensions */
    uint64_t gr = folio_grid_new();
    int32_t gCols[] = {2, 2}; /* fr, fr */
    double gColVals[] = {1.0, 1.0};
    folio_grid_set_template_columns(gr, gCols, gColVals, 2);
    rc = folio_grid_set_border(gr, 1.0, 0.3, 0.3, 0.3);
    ASSERT(rc == 0, "grid_set_border succeeds");
    rc = folio_grid_set_borders(gr,
        1,0,0,0, 1,0,0,0, 1,0,0,0, 1,0,0,0);
    ASSERT(rc == 0, "grid_set_borders succeeds");
    const char* gAreas[] = {"a a", "b c"};
    int32_t gAreaCols[] = {2, 2};
    rc = folio_grid_set_template_areas(gr, gAreas, gAreaCols, 2);
    ASSERT(rc == 0, "grid_set_template_areas succeeds");
    uint64_t gP1 = folio_paragraph_new("A", helv, 10.0);
    folio_grid_add_child(gr, gP1);
    uint64_t gP2 = folio_paragraph_new("B", helv, 10.0);
    folio_grid_add_child(gr, gP2);
    uint64_t gP3 = folio_paragraph_new("C", helv, 10.0);
    folio_grid_add_child(gr, gP3);
    folio_document_add(doc, gr);

    /* Flex extensions */
    uint64_t fl = folio_flex_new();
    rc = folio_flex_set_borders(fl,
        1,0.2,0.2,0.2, 1,0.2,0.2,0.2, 1,0.2,0.2,0.2, 1,0.2,0.2,0.2);
    ASSERT(rc == 0, "flex_set_borders succeeds");
    rc = folio_flex_set_align_content(fl, 2 /* center */);
    ASSERT(rc == 0, "flex_set_align_content succeeds");
    uint64_t fpara = folio_paragraph_new("Flex child", helv, 10.0);
    folio_flex_add(fl, fpara);
    folio_document_add(doc, fl);

    /* Paragraph text-align-last */
    uint64_t tap = folio_paragraph_new("Justified paragraph with align last setting", helv, 12.0);
    rc = folio_paragraph_set_align(tap, 3 /* justify */);
    ASSERT(rc == 0, "para_set_align succeeds");
    rc = folio_paragraph_set_text_align_last(tap, 1 /* center */);
    ASSERT(rc == 0, "para_set_text_align_last succeeds");
    folio_document_add(doc, tap);

    /* Run list with background highlight */
    uint64_t rl62 = folio_run_list_new();
    folio_run_list_add(rl62, "Normal ", helv, 12, 0, 0, 0);
    folio_run_list_add(rl62, "highlighted", helv, 12, 0, 0, 0);
    rc = folio_run_list_last_set_background_color(rl62, 1.0, 1.0, 0.0);
    ASSERT(rc == 0, "run_list_last_set_background_color succeeds");
    folio_run_list_add(rl62, " text.", helv, 12, 0, 0, 0);

    uint64_t runP = folio_heading_new("placeholder", 2);
    folio_heading_set_runs(runP, rl62);
    folio_document_add(doc, runP);

    rc = folio_document_save(doc, "/tmp/folio_cabi_v062.pdf");
    ASSERT(rc == 0, "v062 document save succeeds");

    folio_run_list_free(rl62);
    folio_heading_free(runP);
    folio_flex_free(fl);
    folio_grid_free(gr);
    folio_table_free(tbl62);
    folio_div_free(div);
    folio_document_free(doc);

    /* HTML ParseCSSLength */
    double cssLen = folio_html_parse_css_length("72pt", 12.0, 100.0);
    ASSERT(cssLen == 72.0, "parse_css_length 72pt = 72");
    cssLen = folio_html_parse_css_length("1in", 12.0, 100.0);
    ASSERT(cssLen > 71.9 && cssLen < 72.1, "parse_css_length 1in ≈ 72");
    cssLen = folio_html_parse_css_length("50%", 12.0, 200.0);
    ASSERT(cssLen == 100.0, "parse_css_length 50% of 200 = 100");
    cssLen = folio_html_parse_css_length("2em", 14.0, 100.0);
    ASSERT(cssLen == 28.0, "parse_css_length 2em at 14px = 28");

    /* PKCS#12 signer (negative test — invalid data) */
    uint64_t badPkcs12 = folio_signer_new_pkcs12("invalid", 7, "wrong");
    ASSERT(badPkcs12 == 0, "signer_new_pkcs12 rejects invalid data");

    /* Image element object-fit (exists but no image loaded; test error path) */
    rc = folio_image_element_set_object_fit(99999, "contain");
    ASSERT(rc != 0, "image_element_set_object_fit rejects bad handle");
    rc = folio_image_element_set_object_position(99999, "center");
    ASSERT(rc != 0, "image_element_set_object_position rejects bad handle");

    /* Summary */
    printf("\n%d passed, %d failed\n", passes, failures);
    return failures > 0 ? 1 : 0;
}
