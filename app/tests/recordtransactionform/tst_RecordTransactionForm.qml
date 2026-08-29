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

    // The form is the FIRST amount gate, not the only one — core and the daemon reject a zero
    // amount too. It gates here so a zero, blank, or non-numeric magnitude is answered in the
    // form rather than by a round trip, which is why submit must stay disabled.
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

    // Shapes the keystroke validator refuses but a property write still delivers — and every
    // spec and host writes amountText directly, so the predicate has to refuse them too rather
    // than trusting the validator to have filtered them. Three decimals is the one that
    // corrupted silently: it parsed, passed, and recorded a rounded amount the user never
    // typed. Table-driven because the four shapes share one expectation.
    function test_rejectsAmountShapesItCannotCarry_data() {
        return [
            { tag: "three decimals", text: "12.345" },
            { tag: "exponent overflowing to Infinity", text: "1e400" },
            { tag: "modest exponent", text: "1e3" },
            { tag: "trailing garbage", text: "12.34abc" }
        ]
    }

    function test_rejectsAmountShapesItCannotCarry(data) {
        var ctx = build({ accountId: "a1" })
        ctx.form.nameText = "Coffee"
        ctx.form.amountText = data.text
        // Pins the premise the whole guard rests on: the write LANDS, bypassing the keystroke
        // validator. Without this, all four assertions below would also hold if the write had
        // silently failed — passing for a reason other than the one they name.
        compare(ctx.form.amountText, data.text)
        compare(ctx.form.canSubmit, false)
        compare(ctx.form.signedCents, 0)
        compare(ctx.form.previewText, "")
        ctx.form.submit()
        compare(ctx.mock.recordCallCount, 0)
    }

    // submit() sends signed cents, trimmed name, the date, description, status Reconciled (3),
    // and the injected accountId. lastRecordName and lastRecordDescription are asserted
    // INDEPENDENTLY: the params object removes the positional-transpose hazard, but separate
    // assertions are still what catch a name<->description swap in how the keys are populated.
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

    // The params object trades the positional-transpose hazard for a silent-key one: a mistyped
    // key arrives absent rather than erroring. This pins the exact key set the C++ reader
    // unpacks, which no other spec can reach across the QML/C++ seam.
    function test_submitSendsExpectedParamKeys() {
        var ctx = build({ accountId: "a1" })
        fillValidExpense(ctx)
        ctx.form.submit()
        compare(ctx.mock.lastRecordKeys.join(","),
                "accountId,amount,date,description,name,status")
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

    // A large amount (above the 32-bit int ceiling) records exactly: signedCents is a JS
    // number, not a 32-bit QML int, so ~$30M does not overflow or truncate on the way to the
    // client's 64-bit qlonglong amount.
    function test_largeAmountDoesNotOverflow() {
        var ctx = build({ accountId: "a1" })
        ctx.form.nameText = "House"
        ctx.form.amountText = "30000000" // $30,000,000 -> 3,000,000,000 cents (> INT32_MAX)
        ctx.form.expense = true
        compare(ctx.form.signedCents, -3000000000)
        ctx.form.submit()
        compare(ctx.mock.lastRecordAmount, -3000000000)
    }

    // The largest amount the control accepts — 12 integer digits — records exactly. That bound
    // exists so cents stay well under 2^53, past which a JS number stops representing integers
    // faithfully and the value would drift on its way to a 64-bit sink.
    function test_amountAtCapSubmitsExactCents() {
        var ctx = build({ accountId: "a1" })
        ctx.form.nameText = "Portfolio"
        ctx.form.amountText = "999999999999.99"
        ctx.form.expense = true
        compare(ctx.form.signedCents, -99999999999999)
        ctx.form.submit()
        compare(ctx.mock.lastRecordAmount, -99999999999999)
    }

    // One integer digit past the cap. Set programmatically, since the keystroke validator
    // refuses the 13th digit — which is exactly why the predicate must refuse it independently.
    function test_amountPastCapIsRefused() {
        var ctx = build({ accountId: "a1" })
        ctx.form.nameText = "Too much"
        ctx.form.amountText = "1000000000000"
        compare(ctx.form.amountText, "1000000000000") // the write lands; the predicate refuses it
        compare(ctx.form.canSubmit, false)
        compare(ctx.form.signedCents, 0)
        ctx.form.submit()
        compare(ctx.mock.recordCallCount, 0)
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
