/**
 * Dump semanal do Gmail para a planilha lida pelo opslink weeklyGcp.
 *
 * O bot NÃO acessa Gmail. Este script roda na conta que já tem GmailApp
 * e deixa o digest pronto ANTES do cron do weeklyGcp.
 *
 * Horário obrigatório: segunda 08:45 America/Recife
 * (o CronJob weeklyGcp dispara às 09:00 Recife / 12:00 UTC)
 *
 * Setup:
 * 1. Crie uma planilha com a aba "Emails" e cabeçalho em A1:F1:
 *    fetched_at | subject | from | date | body | links
 * 2. Compartilhe a planilha com a service account usada pelo Opslink (Leitor)
 * 3. Cole o ID da planilha em SHEET_ID
 * 4. Triggers → disparo por horário: segunda 08:45, fuso America/Recife
 *    Função: dumpGmailWeeklyGcp
 */

var SHEET_ID = "COLE_O_ID_DA_PLANILHA";
var GMAIL_QUERY = "newer_than:7d (to:cloud-notices@example.com OR to:platform@example.com)";
var MAX_THREADS = 40;
var BODY_CHARS = 1800;

function dumpGmailWeeklyGcp() {
  var threads = GmailApp.search(GMAIL_QUERY, 0, MAX_THREADS);
  var rows = [];

  threads.forEach(function (thread) {
    thread.getMessages().forEach(function (msg) {
      var body = String(msg.getPlainBody() || "").replace(/\s+/g, " ").substring(0, BODY_CHARS);
      var links = extractUrls(body).join(" ");
      rows.push([
        new Date(),
        msg.getSubject(),
        msg.getFrom(),
        msg.getDate(),
        body,
        links
      ]);
    });
  });

  var sheet = SpreadsheetApp.openById(SHEET_ID).getSheetByName("Emails");
  if (!sheet) {
    throw new Error('Aba "Emails" não encontrada');
  }

  if (sheet.getLastRow() > 1) {
    sheet.getRange(2, 1, sheet.getLastRow() - 1, 6).clearContent();
  }

  if (rows.length > 0) {
    sheet.getRange(2, 1, rows.length, 6).setValues(rows);
  }
}

function extractUrls(text) {
  var seen = {};
  var out = [];
  var re = /https?:\/\/[^\s<>"')\]]+/g;
  var match;
  while ((match = re.exec(text)) !== null) {
    var url = match[0].replace(/[.,;:)]+$/, "");
    if (!seen[url]) {
      seen[url] = true;
      out.push(url);
    }
  }
  return out;
}
