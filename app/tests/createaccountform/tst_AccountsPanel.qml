import QtQuick
import QtTest
// Same test-only module as the form spec: it now also carries AccountsPanel, which
// embeds the real CreateAccountForm. The mock stands in for the daemon-backed client.
import FinchFormTest

TestCase {
    id: testCase
    name: "AccountsPanel"
    when: windowShown
    width: 400
    height: 400

    Component {
        id: panelFactory
        AccountsPanel {}
    }

    Component {
        id: mockFactory
        MockFinchClient {}
    }

    property var panel: null
    property var mock: null

    function buildPanel(createEnabled) {
        mock = mockFactory.createObject(testCase)
        panel = panelFactory.createObject(testCase, { client: mock, createEnabled: createEnabled })
        verify(panel !== null, "panel instantiated")
        return panel
    }

    function cleanup() {
        if (panel) { panel.destroy(); panel = null }
        if (mock) { mock.destroy(); mock = null }
    }

    // The connection gate (issue #76's must-preserve behavior) propagates to the embedded
    // create form: closed gate disables it. This was previously enforced by the toolbar
    // button that opened the now-removed modal dialog.
    function test_formDisabledWhenGateClosed() {
        var p = buildPanel(false)
        var form = findChild(p, "createForm")
        verify(form !== null, "embedded create form found")
        compare(form.enabled, false)
    }

    function test_formEnabledWhenGateOpen() {
        var p = buildPanel(true)
        var form = findChild(p, "createForm")
        verify(form !== null, "embedded create form found")
        compare(form.enabled, true)
    }
}
