#ifndef FINCHCLIENT_H
#define FINCHCLIENT_H

#include <QElapsedTimer>
#include <QFutureWatcher>
#include <QMap>
#include <QObject>
#include <QPointF>
#include <QString>
#include <QStringList>
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

struct CreateAccountResult {
    bool ok = false;
    QString id;
    QString name;
    QString errorMessage;
};

struct ListTransactionsResult {
    bool ok = false;
    QVariantList transactions;
};

struct TimeSeriesPoint {
    qint64 msecsSinceEpoch;
    double balance;
};

struct TimeSeriesResult {
    bool ok = false;
    QMap<QString, QList<TimeSeriesPoint>> seriesByAccount;
};

class FinchClient : public QObject {
    Q_OBJECT
    Q_PROPERTY(QString daemonVersion READ daemonVersion NOTIFY daemonVersionChanged)
    Q_PROPERTY(ConnectionState connectionState READ connectionState NOTIFY connectionStateChanged)
    Q_PROPERTY(bool pingInProgress READ pingInProgress NOTIFY pingInProgressChanged)
    Q_PROPERTY(QVariantList accounts READ accounts NOTIFY accountsChanged)
    Q_PROPERTY(bool accountsLoading READ accountsLoading NOTIFY accountsLoadingChanged)
    Q_PROPERTY(bool createAccountInProgress READ createAccountInProgress NOTIFY createAccountInProgressChanged)
    Q_PROPERTY(QVariantList transactions READ transactions NOTIFY transactionsChanged)
    Q_PROPERTY(bool transactionsLoading READ transactionsLoading NOTIFY transactionsLoadingChanged)
    Q_PROPERTY(bool timeSeriesLoading READ timeSeriesLoading NOTIFY timeSeriesLoadingChanged)
    Q_PROPERTY(bool timeSeriesEmpty READ timeSeriesEmpty NOTIFY timeSeriesDataChanged)
    Q_PROPERTY(QStringList timeSeriesAccountIds READ timeSeriesAccountIds NOTIFY timeSeriesDataChanged)
    Q_PROPERTY(double timeSeriesMinDate READ timeSeriesMinDate NOTIFY timeSeriesDataChanged)
    Q_PROPERTY(double timeSeriesMaxDate READ timeSeriesMaxDate NOTIFY timeSeriesDataChanged)
    Q_PROPERTY(double timeSeriesMinBalance READ timeSeriesMinBalance NOTIFY timeSeriesDataChanged)
    Q_PROPERTY(double timeSeriesMaxBalance READ timeSeriesMaxBalance NOTIFY timeSeriesDataChanged)

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
    bool createAccountInProgress() const { return m_createAccountInProgress; }
    QVariantList transactions() const { return m_transactions; }
    bool transactionsLoading() const { return m_transactionsLoading; }
    bool timeSeriesLoading() const { return m_timeSeriesLoading; }
    bool timeSeriesEmpty() const;
    QStringList timeSeriesAccountIds() const;
    double timeSeriesMinDate() const;
    double timeSeriesMaxDate() const;
    double timeSeriesMinBalance() const;
    double timeSeriesMaxBalance() const;

    Q_INVOKABLE void ping();
    Q_INVOKABLE void listAccounts();
    Q_INVOKABLE void createAccount(const QString& name, int accountType);
    Q_INVOKABLE void listTransactions(const QString& accountId);
    Q_INVOKABLE void fetchTimeSeries(const QString& fromDate, const QString& toDate,
                                     int interval, const QStringList& accountIds);
    Q_INVOKABLE void populateSeries(QObject* series, const QString& accountId);

signals:
    void daemonVersionChanged();
    void connectionStateChanged();
    void pingInProgressChanged();
    void accountsChanged();
    void accountsLoadingChanged();
    void createAccountInProgressChanged();
    void accountCreated(QString id);
    void accountCreateFailed(QString message);
    void transactionsChanged();
    void transactionsLoadingChanged();
    void timeSeriesLoadingChanged();
    void timeSeriesDataChanged();

private slots:
    void onPingFinished();
    void applyPingResult();
    void onListAccountsFinished();
    void onCreateAccountFinished();
    void onListTransactionsFinished();
    void onTimeSeriesFinished();

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
    bool m_accountsRefreshPending = false;
    QFutureWatcher<ListAccountsResult> m_accountsWatcher;
    bool m_createAccountInProgress = false;
    QFutureWatcher<CreateAccountResult> m_createAccountWatcher;
    QVariantList m_transactions;
    bool m_transactionsLoading = false;
    QFutureWatcher<ListTransactionsResult> m_transactionsWatcher;
    bool m_timeSeriesLoading = false;
    QFutureWatcher<TimeSeriesResult> m_timeSeriesWatcher;
    TimeSeriesResult m_timeSeriesData;
};

#endif // FINCHCLIENT_H
