#include <QApplication>
#include <QIcon>
#include <QQmlApplicationEngine>
#include <QQmlContext>
#include <QUrl>
#include <QtQml>

#include "finchclient.h"

#include <cstdlib>
#include <string>
#include <unistd.h>

static std::string socketPath()
{
    const char* xdgRuntime = std::getenv("XDG_RUNTIME_DIR");
    if (xdgRuntime) {
        return std::string(xdgRuntime) + "/finch/finch.sock";
    }
    // macOS fallback
    return "/tmp/finch-" + std::to_string(getuid()) + "/finch.sock";
}

int main(int argc, char* argv[])
{
    QApplication app(argc, argv);
    app.setApplicationName("Finch");
    app.setApplicationVersion("0.1.0");
    // Wayland derives a window's app_id from the desktop file name; matching it to
    // finch.desktop is what lets the compositor resolve the taskbar/window icon.
    app.setDesktopFileName(QStringLiteral("finch"));
    app.setWindowIcon(QIcon::fromTheme(QStringLiteral("finch")));

    qmlRegisterUncreatableType<FinchClient>("Finch", 1, 0, "FinchClient",
                                            "FinchClient is not creatable from QML");

    auto client = new FinchClient(QString::fromStdString(socketPath()), &app);

    QQmlApplicationEngine engine;
    engine.rootContext()->setContextProperty("finchClient", client);
    engine.load(QUrl(QStringLiteral("qrc:/Finch/qml/Main.qml")));

    if (engine.rootObjects().isEmpty())
        return -1;

    client->ping();

    return app.exec();
}
