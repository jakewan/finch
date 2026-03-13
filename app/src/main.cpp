#include <QGuiApplication>
#include <QQmlApplicationEngine>
#include <QQmlContext>
#include <QUrl>

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
    QGuiApplication app(argc, argv);
    app.setApplicationName("Finch");
    app.setApplicationVersion("0.1.0");

    auto client = new FinchClient(QString::fromStdString(socketPath()), &app);
    client->ping();

    QQmlApplicationEngine engine;
    engine.rootContext()->setContextProperty("finchClient", client);
    engine.load(QUrl(QStringLiteral("qrc:/Finch/qml/Main.qml")));

    if (engine.rootObjects().isEmpty())
        return -1;

    return app.exec();
}
