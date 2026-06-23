import QtQuick
import QtTest
// FinchRecordTest is the test-only module (see app/CMakeLists.txt) holding the real shipping
// RecordTransactionForm, the txformat.js helper it imports, and the shared MockFinchClient
// double — scoped to just those files so the test doesn't pull in the rest of the app's QML.
import FinchRecordTest

TestCase {
    id: testCase
    name: "RecordTransactionForm"
    when: windowShown

    Component {
        id: formFactory
        RecordTransactionForm {}
    }

    Component {
        id: mockFactory
        MockFinchClient {}
    }

    SignalSpy {
        id: recordedSpy
        signalName: "recorded"
    }

    property var currentForm: null
    property var currentMock: null

    // Returns { form, mock }; cleanup() destroys them after each test — even on an assertion
    // failure, which aborts the test function before any manual teardown runs. accountId is
    // injected (the panel feeds it from the screen's selector) so build() takes it as an opt.
    function build(opts) {
        opts = opts || {}
        currentMock = mockFactory.createObject(testCase)
        var props = { client: currentMock }
        if (opts.accountId !== undefined)
            props.accountId = opts.accountId
        currentForm = formFactory.createObject(testCase, props)
        verify(currentForm !== null, "form instantiated")
        return { form: currentForm, mock: currentMock }
    }

    function cleanup() {
        recordedSpy.target = null
        if (currentForm) { currentForm.destroy(); currentForm = null }
        if (currentMock) { currentMock.destroy(); currentMock = null }
    }

    // Drives a valid Expense in one step: account injected, name + magnitude set.
    function fillValidExpense(ctx) {
        ctx.form.accountId = "a1"
        ctx.form.nameText = "Coffee"
        ctx.form.amountText = "12.34"
    }

    function test_loadsAndStartsInvalid() {
        var ctx = build()
        compare(ctx.form.canSubmit, false)
    }

    function test_enabledOnceAccountNameAmountPresent() {
        var ctx = build()
        fillValidExpense(ctx)
        compare(ctx.form.canSubmit, true)
    }

    // No account selected (injected accountId empty) blocks submit even with name + amount.
    function test_disabledWhenNoAccount() {
        var ctx = build()
        ctx.form.nameText = "Coffee"
        ctx.form.amountText = "12.34"
        compare(ctx.form.accountId, "")
        compare(ctx.form.canSubmit, false)
    }

    function test_disabledWhenNameBlank() {
        var ctx = build({ accountId: "a1" })
        ctx.form.amountText = "12.34"
        ctx.form.nameText = "   " // whitespace only
        compare(ctx.form.canSubmit, false)
    }

    // The form is the SOLE amount gate — the daemon does not validate amount at all — so a
    // zero, blank, or non-numeric magnitude must keep submit disabled.
    function test_disabledWhenAmountZero() {
        var ctx = build({ accountId: "a1" })
        ctx.form.nameText = "Coffee"
        ctx.form.amountText = "0"
        compare(ctx.form.canSubmit, false)
    }

    function test_disabledWhenAmountBlank() {
        var ctx = build({ accountId: "a1" })
        ctx.form.nameText = "Coffee"
        ctx.form.amountText = ""
        compare(ctx.form.canSubmit, false)
    }

    function test_disabledWhenAmountNonNumeric() {
        var ctx = build({ accountId: "a1" })
        ctx.form.nameText = "Coffee"
        ctx.form.amountText = "abc"
        compare(ctx.form.canSubmit, false)
    }

    // submit() sends signed cents, trimmed name, the date, description, status Reconciled (3),
    // and the injected accountId. lastRecordName and lastRecordDescription are asserted
    // INDEPENDENTLY — a 6-arg positional Q_INVOKABLE with two adjacent strings is a transpose
    // waiting to happen, and only separate assertions catch a name<->description swap.
    function test_submitSendsExpenseAsNegativeCents() {
        var ctx = build({ accountId: "a1" })
        ctx.form.nameText = "  Coffee  "
        ctx.form.amountText = "12.34"
        ctx.form.expense = true
        ctx.form.dateText = "2026-06-22"
        ctx.form.descriptionText = "morning latte"
        ctx.form.submit()
        compare(ctx.mock.recordCallCount, 1)
        compare(ctx.mock.lastRecordAccountId, "a1")
        compare(ctx.mock.lastRecordDate, "2026-06-22")
        compare(ctx.mock.lastRecordAmount, -1234)
        compare(ctx.mock.lastRecordName, "Coffee")
        compare(ctx.mock.lastRecordDescription, "morning latte")
        compare(ctx.mock.lastRecordStatus, 3)
    }

    function test_submitSendsIncomeAsPositiveCents() {
        var ctx = build({ accountId: "a1" })
        ctx.form.nameText = "Paycheck"
        ctx.form.amountText = "50"
        ctx.form.expense = false // Income
        ctx.form.submit()
        compare(ctx.mock.recordCallCount, 1)
        compare(ctx.mock.lastRecordAmount, 5000)
    }

    // Sign integrity: the toggle is the sole sign source. The validator forbids a leading "-",
    // so whatever the user types, an Expense is never positive and an Income never negative.
    function test_signIntegrity() {
        var ctx = build({ accountId: "a1" })
        ctx.form.nameText = "Thing"
        ctx.form.amountText = "9.99"
        ctx.form.expense = true
        verify(ctx.form.signedCents < 0)
        ctx.form.expense = false
        verify(ctx.form.signedCents > 0)
    }

    // Preview shows the signed amount and flips with the toggle; empty when magnitude invalid.
    function test_previewReflectsSignAndMagnitude() {
        var ctx = build({ accountId: "a1" })
        ctx.form.amountText = "12.34"
        ctx.form.expense = true
        compare(ctx.form.previewText, "-$12.34")
        ctx.form.expense = false
        compare(ctx.form.previewText, "$12.34")
    }

    function test_previewEmptyWhenAmountInvalid() {
        var ctx = build({ accountId: "a1" })
        ctx.form.amountText = ""
        compare(ctx.form.previewText, "")
        ctx.form.amountText = "abc"
        compare(ctx.form.previewText, "")
        ctx.form.amountText = "0"
        compare(ctx.form.previewText, "")
    }

    function test_submitIgnoredWhenInvalid() {
        var ctx = build({ accountId: "a1" })
        ctx.form.nameText = "   "
        ctx.form.amountText = "12.34"
        ctx.form.submit()
        compare(ctx.mock.recordCallCount, 0)
    }

    function test_successClearsFieldsAndSignalsRecorded() {
        var ctx = build({ accountId: "a1" })
        recordedSpy.target = ctx.form
        recordedSpy.clear()
        ctx.form.nameText = "Coffee"
        ctx.form.amountText = "12.34"
        ctx.form.descriptionText = "latte"
        ctx.form.submit()
        ctx.mock.succeedRecord("txn-1")
        compare(recordedSpy.count, 1)
        compare(ctx.form.nameText, "")
        compare(ctx.form.amountText, "")
        compare(ctx.form.descriptionText, "")
        recordedSpy.target = null
    }

    function test_failureShowsErrorMessage() {
        var ctx = build({ accountId: "a1" })
        ctx.form.nameText = "Coffee"
        ctx.form.amountText = "12.34"
        ctx.form.submit()
        ctx.mock.failRecord("name must not be empty")
        compare(ctx.form.errorText, "name must not be empty")
    }

    function test_disabledWhileRecordInProgress() {
        var ctx = build({ accountId: "a1" })
        ctx.form.nameText = "Coffee"
        ctx.form.amountText = "12.34"
        verify(ctx.form.canSubmit) // enabled before submit
        ctx.form.submit() // mock latches recordTransactionInProgress synchronously
        compare(ctx.mock.recordTransactionInProgress, true)
        compare(ctx.form.canSubmit, false)
    }

    function test_doubleSubmitSendsOnlyOne() {
        var ctx = build({ accountId: "a1" })
        ctx.form.nameText = "Coffee"
        ctx.form.amountText = "12.34"
        ctx.form.submit()
        ctx.form.submit() // second gated by canSubmit (in-progress)
        compare(ctx.mock.recordCallCount, 1)
    }
}
