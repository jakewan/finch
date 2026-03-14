#ifndef FINCHCLIENT_H
#define FINCHCLIENT_H

#include <QElapsedTimer>
#include <QFutureWatcher>
#include <QObject>
#include <QString>
#include <QVariantList>
#include <memory>

#include <grpcpp/grpcpp.h>
#include "finch/v1/finch.grpc.pb.h"

struct PingResult {
    bool ok;
    QString version;
};

struct ListAccountsResult {
    bool ok;
    QVariantList accounts;
};

class FinchClient : public QObject {
    Q_OBJECT
    Q_PROPERTY(QString daemonVersion READ daemonVersion NOTIFY daemonVersionChanged)
    Q_PROPERTY(ConnectionState connectionState READ connectionState NOTIFY connectionStateChanged)
    Q_PROPERTY(bool pingInProgress READ pingInProgress NOTIFY pingInProgressChanged)
    Q_PROPERTY(QVariantList accounts READ accounts NOTIFY accountsChanged)
    Q_PROPERTY(bool accountsLoading READ accountsLoading NOTIFY accountsLoadingChanged)

public:
    enum ConnectionState { Disconnected, Connecting, Connected };
    Q_ENUM(ConnectionState)

    explicit FinchClient(const QString& socketPath, QObject* parent = nullptr);
    ~FinchClient() override;

    QString daemonVersion() const { return m_version; }
    ConnectionState connectionState() const { return m_connectionState; }
    bool pingInProgress() const { return m_pingInProgress; }
    QVariantList accounts() const { return m_accounts; }
    bool accountsLoading() const { return m_accountsLoading; }

    Q_INVOKABLE void ping();
    Q_INVOKABLE void listAccounts();

signals:
    void daemonVersionChanged();
    void connectionStateChanged();
    void pingInProgressChanged();
    void accountsChanged();
    void accountsLoadingChanged();

private slots:
    void onPingFinished();
    void applyPingResult();
    void onListAccountsFinished();

private:
    std::shared_ptr<grpc::Channel> m_channel;
    std::unique_ptr<finch::v1::FinchService::Stub> m_stub;
    QString m_version;
    ConnectionState m_connectionState = Disconnected;
    bool m_pingInProgress = false;
    QFutureWatcher<PingResult> m_pingWatcher;
    QElapsedTimer m_connectingTimer;
    PingResult m_pendingResult;
    QVariantList m_accounts;
    bool m_accountsLoading = false;
    QFutureWatcher<ListAccountsResult> m_accountsWatcher;
};

#endif // FINCHCLIENT_H
