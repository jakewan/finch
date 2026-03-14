#ifndef FINCHCLIENT_H
#define FINCHCLIENT_H

#include <QElapsedTimer>
#include <QFutureWatcher>
#include <QObject>
#include <QString>
#include <memory>

#include <grpcpp/grpcpp.h>
#include "finch/v1/finch.grpc.pb.h"

struct PingResult {
    bool ok;
    QString version;
};

class FinchClient : public QObject {
    Q_OBJECT
    Q_PROPERTY(QString daemonVersion READ daemonVersion NOTIFY daemonVersionChanged)
    Q_PROPERTY(ConnectionState connectionState READ connectionState NOTIFY connectionStateChanged)
    Q_PROPERTY(bool pingInProgress READ pingInProgress NOTIFY pingInProgressChanged)

public:
    enum ConnectionState { Disconnected, Connecting, Connected };
    Q_ENUM(ConnectionState)

    explicit FinchClient(const QString& socketPath, QObject* parent = nullptr);
    ~FinchClient() override;

    QString daemonVersion() const { return m_version; }
    ConnectionState connectionState() const { return m_connectionState; }
    bool pingInProgress() const { return m_pingInProgress; }

    Q_INVOKABLE void ping();

signals:
    void daemonVersionChanged();
    void connectionStateChanged();
    void pingInProgressChanged();

private slots:
    void onPingFinished();
    void applyPingResult();

private:
    std::shared_ptr<grpc::Channel> m_channel;
    std::unique_ptr<finch::v1::FinchService::Stub> m_stub;
    QString m_version;
    ConnectionState m_connectionState = Disconnected;
    bool m_pingInProgress = false;
    QFutureWatcher<PingResult> m_pingWatcher;
    QElapsedTimer m_connectingTimer;
    PingResult m_pendingResult;
};

#endif // FINCHCLIENT_H
