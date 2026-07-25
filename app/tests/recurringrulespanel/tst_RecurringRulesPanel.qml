import QtQuick
import QtTest
// FinchRecurringPanelTest is the test-only module (see app/CMakeLists.txt) holding the real
// shipping RecurringRulesPanel, the CreateRecurringRuleForm it embeds, the txformat.js helper,
// and the shared MockFinchClient double — scoped to just those files.
import FinchRecurringPanelTest

TestCase {
    id: testCase
    name: "RecurringRulesPanel"
    when: windowShown
    width: 640
    height: 480

    Component { id: panelFactory; RecurringRulesPanel {} }
    Component { id: mockFactory; MockFinchClient {} }

    property var currentPanel: null
    property var currentMock: null

    readonly property var sampleAccounts: [
        { id: "a1", name: "Checking" },
        { id: "a2", name: "Savings" }
    ]

    // A monthly expense and a semi-monthly paused credit, so one fixture pins the amount
    // format, the frequency->label mapping, the schedule text, and the paused flag.
    readonly property var sampleRules: [
        { id: "r1", accountId: "a1", name: "Rent", amount: -150000, frequency: 4,
          startDate: "2025-01-01", endDate: "", dayOfMonth: 1, semiMonthlyDays: [],
          isTransfer: false, transferTargetAccountId: "", paused: false },
        { id: "r2", accountId: "a1", name: "Paycheck", amount: 200000, frequency: 3,
          startDate: "2025-01-01", endDate: "2025-12-31", dayOfMonth: 0, semiMonthlyDays: [1, 15],
          isTransfer: false, transferTargetAccountId: "", paused: true }
    ]

    // Accounts are set on the mock BEFORE the panel is built, so the selector model is populated
    // at completion — the only way to prove the panel does not auto-fetch on a non-empty list.
    function build(opts) {
        opts = opts || {}
        currentMock = mockFactory.createObject(testCase)
        currentMock.accounts = (opts.accounts !== undefined) ? opts.accounts : sampleAccounts
        var props = { client: currentMock }
        props.connected = (opts.connected !== undefined) ? opts.connected : true
        currentPanel = panelFactory.createObject(testCase, props)
        verify(currentPanel !== null, "panel instantiated")
        return { panel: currentPanel, mock: currentMock }
    }

    function cleanup() {
        if (currentPanel) { currentPanel.destroy(); currentPanel = null }
        if (currentMock) { currentMock.destroy(); currentMock = null }
    }

    function selectAccountWith(ctx, index, rules) {
        ctx.panel.selectedAccountIndex = index
        ctx.mock.succeedRecurringRules(rules)
    }

    function test_loadsAndStartsUnselected() {
        var ctx = build()
        compare(ctx.panel.selectedAccountIndex, -1)
        compare(ctx.mock.recurringRulesCallCount, 0)
    }

    function test_noFetchUntilAccountSelected() {
        var ctx = build()
        compare(ctx.mock.recurringRulesCallCount, 0)
        ctx.panel.selectedAccountIndex = 0
        compare(ctx.mock.recurringRulesCallCount, 1)
        compare(ctx.mock.lastRulesAccountId, "a1")
    }

    function test_selectingAccountListsThatAccount() {
        var ctx = build()
        ctx.panel.selectedAccountIndex = 1
        compare(ctx.mock.recurringRulesCallCount, 1)
        compare(ctx.mock.lastRulesAccountId, "a2")
    }

    function test_changingAccountRefetches() {
        var ctx = build()
        ctx.panel.selectedAccountIndex = 0
        ctx.panel.selectedAccountIndex = 1
        compare(ctx.mock.recurringRulesCallCount, 2)
        compare(ctx.mock.lastRulesAccountId, "a2")
    }

    function test_rulesRenderAsRows() {
        var ctx = build()
        selectAccountWith(ctx, 0, sampleRules)
        compare(ctx.panel.ruleList.count, 2)
    }

    function test_selectingRowFillsDetail() {
        var ctx = build()
        selectAccountWith(ctx, 0, sampleRules)
        ctx.panel.ruleList.currentIndex = 0
        verify(ctx.panel.selectedRule !== null)
        compare(ctx.panel.detailNameText, "Rent")
        compare(ctx.panel.detailStartDateText, "2025-01-01")
    }

    function test_amountFormattedWithSign() {
        var ctx = build()
        selectAccountWith(ctx, 0, sampleRules)
        ctx.panel.ruleList.currentIndex = 0
        compare(ctx.panel.detailAmountText, "-$1500.00")
    }

    function test_frequencyRenderedAsLabelNotInt() {
        var ctx = build()
        selectAccountWith(ctx, 0, sampleRules)
        ctx.panel.ruleList.currentIndex = 0
        compare(ctx.panel.detailFrequencyText, "Monthly")
        ctx.panel.ruleList.currentIndex = 1
        compare(ctx.panel.detailFrequencyText, "Semi-monthly")
    }

    // Schedule text reads the frequency-conditional day fields: a day-of-month for Monthly,
    // the two days for Semi-monthly.
    function test_scheduleTextReflectsFrequency() {
        var ctx = build()
        selectAccountWith(ctx, 0, sampleRules)
        ctx.panel.ruleList.currentIndex = 0
        compare(ctx.panel.detailScheduleText, "Day 1")
        ctx.panel.ruleList.currentIndex = 1
        compare(ctx.panel.detailScheduleText, "Days 1 & 15")
    }

    function test_pausedRuleShown() {
        var ctx = build()
        selectAccountWith(ctx, 0, sampleRules)
        ctx.panel.ruleList.currentIndex = 0
        compare(ctx.panel.detailStatusText, "Active")
        ctx.panel.ruleList.currentIndex = 1
        compare(ctx.panel.detailStatusText, "Paused")
    }

    function test_endDateShownOrDash() {
        var ctx = build()
        selectAccountWith(ctx, 0, sampleRules)
        ctx.panel.ruleList.currentIndex = 0
        compare(ctx.panel.detailEndDateText, "—")   // open-ended
        ctx.panel.ruleList.currentIndex = 1
        compare(ctx.panel.detailEndDateText, "2025-12-31")
    }

    function test_emptyResultShowsNoRules() {
        var ctx = build()
        selectAccountWith(ctx, 0, [])
        verify(ctx.panel.stateMessageVisible)
        compare(ctx.panel.stateMessageText, "No recurring rules for this account.")
    }

    function test_noAccountSelectedShowsPrompt() {
        var ctx = build()
        verify(ctx.panel.stateMessageVisible)
        compare(ctx.panel.stateMessageText, "Select an account to view its recurring rules.")
    }

    function test_disconnectedShowsConnectHint() {
        var ctx = build({ connected: false, accounts: [] })
        verify(ctx.panel.stateMessageVisible)
        compare(ctx.panel.stateMessageText, "Connect to the daemon to view recurring rules.")
    }

    function test_changingAccountResetsSelection() {
        var ctx = build()
        selectAccountWith(ctx, 0, sampleRules)
        ctx.panel.ruleList.currentIndex = 0
        verify(ctx.panel.selectedRule !== null)
        ctx.panel.selectedAccountIndex = 1
        compare(ctx.panel.ruleList.currentIndex, -1)
        verify(ctx.panel.selectedRule === null)
        ctx.mock.succeedRecurringRules([sampleRules[1]])
        compare(ctx.panel.ruleList.count, 1)
        compare(ctx.panel.ruleList.currentIndex, -1)
        verify(ctx.panel.selectedRule === null)
    }

    function test_switchingAccountClearsPriorRowsWhileLoading() {
        var ctx = build()
        selectAccountWith(ctx, 0, sampleRules)
        compare(ctx.panel.ruleList.count, 2)
        ctx.panel.selectedAccountIndex = 1
        verify(ctx.mock.recurringRulesLoading)
        compare(ctx.panel.ruleList.count, 0)
    }

    // Rapid account switch: selecting B while A's fetch is in flight must end showing B's rules.
    function test_midFetchSwitchReconcilesToRequestedAccount() {
        var ctx = build()
        ctx.panel.selectedAccountIndex = 0
        verify(ctx.mock.recurringRulesLoading)
        ctx.panel.selectedAccountIndex = 1
        compare(ctx.mock.lastRulesAccountId, "a2")
        ctx.mock.succeedRecurringRules(sampleRules)     // A completes; must be discarded
        verify(ctx.mock.recurringRulesLoading)          // B re-fetch in flight
        compare(ctx.panel.ruleList.count, 0)
        ctx.mock.succeedRecurringRules([sampleRules[1]]) // B completes
        verify(!ctx.mock.recurringRulesLoading)
        compare(ctx.panel.ruleList.count, 1)
    }

    function test_newDataResetsSelection() {
        var ctx = build()
        selectAccountWith(ctx, 0, sampleRules)
        ctx.panel.ruleList.currentIndex = 1
        ctx.mock.succeedRecurringRules([])
        compare(ctx.panel.ruleList.currentIndex, -1)
        verify(ctx.panel.selectedRule === null)
    }

    function test_createFormDisabledBeforeAccountSelected() {
        var ctx = build()
        compare(ctx.panel.createForm.canSubmit, false)
    }

    function test_createFormEnabledAfterAccountAndFields() {
        var ctx = build()
        ctx.panel.selectedAccountIndex = 0
        ctx.panel.createForm.nameText = "Gym"
        ctx.panel.createForm.amountText = "40"
        ctx.panel.createForm.frequencyIndex = 0   // Weekly
        compare(ctx.panel.createForm.canSubmit, true)
    }

    // Driving the embedded form's success path re-fetches the selected account's rules.
    function test_creatingRefetchesSelectedAccount() {
        var ctx = build()
        selectAccountWith(ctx, 0, sampleRules)
        var before = ctx.mock.recurringRulesCallCount
        ctx.panel.createForm.nameText = "Gym"
        ctx.panel.createForm.amountText = "40"
        ctx.panel.createForm.frequencyIndex = 0
        ctx.panel.createForm.submit()
        ctx.mock.succeedCreateRecurringRule("rule-99")
        compare(ctx.mock.recurringRulesCallCount, before + 1)
        compare(ctx.mock.lastRulesAccountId, "a1")
    }

    // --- Transfer rules --------------------------------------------------------------------

    // A transfer rule stores a POSITIVE magnitude and belongs to its source account; direction
    // comes from which side of the transfer the viewing account is on. Rendering the stored
    // amount as-is would show a $500 outbound transfer as a credit on the account it debits.
    readonly property var transferRule: {
        return { id: "r3", accountId: "a1", name: "To savings", amount: 50000, frequency: 4,
                 startDate: "2025-01-01", endDate: "", dayOfMonth: 1, semiMonthlyDays: [],
                 isTransfer: true, transferTargetAccountId: "a2", paused: false }
    }

    // Called by the ListView delegate, so asserting it exercises real production code. Reading
    // a delegate's contents instead would depend on viewport geometry no spec in this suite
    // relies on; the row's visual confirmation belongs to manual testing.
    function test_rowLabelNamesTransferDestination() {
        var ctx = build()
        selectAccountWith(ctx, 0, [transferRule])
        compare(ctx.panel.rowLabel(transferRule), "To savings → Savings")
    }

    function test_rowAmountIsDebitForSourceAccount() {
        var ctx = build()
        selectAccountWith(ctx, 0, [transferRule])
        compare(ctx.panel.ruleAmountText(transferRule), "-$500.00")
    }

    function test_nonTransferRowKeepsStoredSign() {
        var ctx = build()
        selectAccountWith(ctx, 0, sampleRules)
        compare(ctx.panel.rowLabel(sampleRules[0]), "Rent")
        compare(ctx.panel.ruleAmountText(sampleRules[0]), "-$1500.00")
    }

    function test_detailPaneShowsTransferDirectionAndDestination() {
        var ctx = build()
        selectAccountWith(ctx, 0, [transferRule])
        ctx.panel.ruleList.currentIndex = 0
        compare(ctx.panel.detailAmountText, "-$500.00")
        verify(ctx.panel.detailTransferVisible)
        compare(ctx.panel.detailTransferText, "Out to Savings")
    }

    function test_detailPaneHidesTransferRowForOrdinaryRule() {
        var ctx = build()
        selectAccountWith(ctx, 0, sampleRules)
        ctx.panel.ruleList.currentIndex = 0
        compare(ctx.panel.detailTransferVisible, false)
    }

    // Mirrors core's effectOn, which negates only when a transfer has a target. A rule flagged
    // as a transfer with no target falls through to its stored signed amount, so the panel and
    // the projection engine cannot disagree about direction on the same rule.
    function test_transferWithoutTargetKeepsStoredSign() {
        var ctx = build()
        var orphan = { id: "r4", accountId: "a1", name: "Odd", amount: 50000, frequency: 4,
                       startDate: "2025-01-01", endDate: "", dayOfMonth: 1, semiMonthlyDays: [],
                       isTransfer: true, transferTargetAccountId: "", paused: false }
        selectAccountWith(ctx, 0, [orphan])
        compare(ctx.panel.ruleAmountText(orphan), "$500.00")
        compare(ctx.panel.rowLabel(orphan), "Odd")
    }

    // An unknown destination falls back to the raw id — never "undefined" beside a money amount.
    function test_unknownDestinationFallsBackToId() {
        var ctx = build()
        var dangling = { id: "r5", accountId: "a1", name: "Sweep", amount: 50000, frequency: 4,
                         startDate: "2025-01-01", endDate: "", dayOfMonth: 1, semiMonthlyDays: [],
                         isTransfer: true, transferTargetAccountId: "a9", paused: false }
        selectAccountWith(ctx, 0, [dangling])
        compare(ctx.panel.rowLabel(dangling), "Sweep → a9")
    }

    // Viewed from the account a transfer CREDITS, every rendering must flip: the amount is a
    // credit, the row points inward, and the detail pane names the payer rather than the payee.
    // Not reachable through the real client yet (rules are fetched scoped to their owner), but
    // the amount already derives direction from the viewing account, so the labels beside it
    // must agree rather than reading as outbound regardless.
    function test_transferViewedFromDestinationReadsInbound() {
        var ctx = build()
        selectAccountWith(ctx, 1, [transferRule])   // index 1 is Savings, the rule's target
        compare(ctx.panel.currentAccountId, "a2")
        compare(ctx.panel.ruleAmountText(transferRule), "$500.00")
        compare(ctx.panel.rowLabel(transferRule), "To savings ← Checking")
        ctx.panel.ruleList.currentIndex = 0
        verify(ctx.panel.detailTransferVisible)
        compare(ctx.panel.detailTransferText, "In from Checking")
    }

    // "Is this a transfer with a destination" must mean one thing everywhere. The row label and
    // the amount both treat a missing target as not-a-transfer; the detail pane must agree,
    // rather than announcing a transfer to an account it cannot name.
    function test_detailPaneHidesTransferRowWhenTargetMissing() {
        var ctx = build()
        var noTarget = { id: "r7", accountId: "a1", name: "Odd", amount: 50000, frequency: 4,
                         startDate: "2025-01-01", endDate: "", dayOfMonth: 1, semiMonthlyDays: [],
                         isTransfer: true, paused: false }
        selectAccountWith(ctx, 0, [noTarget])
        ctx.panel.ruleList.currentIndex = 0
        compare(ctx.panel.detailTransferVisible, false)
        compare(ctx.panel.detailTransferText, "")
    }

    function test_pausedTransferShowsBothAnnotations() {
        var ctx = build()
        var paused = { id: "r6", accountId: "a1", name: "To savings", amount: 50000, frequency: 4,
                       startDate: "2025-01-01", endDate: "", dayOfMonth: 1, semiMonthlyDays: [],
                       isTransfer: true, transferTargetAccountId: "a2", paused: true }
        selectAccountWith(ctx, 0, [paused])
        compare(ctx.panel.rowLabel(paused), "To savings → Savings (paused)")
    }
}
