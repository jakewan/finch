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
#include <QVariantMap>
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

struct RecordTransactionResult {
    bool ok = false;
    QString id;
    QString errorMessage;
};

struct CreateRecurringRuleResult {
    bool ok = false;
    QString id;
    QString errorMessage;
};

struct ListRecurringRulesResult {
    bool ok = false;
    QVariantList rules;
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
    Q_PROPERTY(bool recordTransactionInProgress READ recordTransactionInProgress NOTIFY recordTransactionInProgressChanged)
    Q_PROPERTY(bool createRecurringRuleInProgress READ createRecurringRuleInProgress NOTIFY createRecurringRuleInProgressChanged)
    Q_PROPERTY(QVariantList recurringRules READ recurringRules NOTIFY recurringRulesChanged)
    Q_PROPERTY(bool recurringRulesLoading READ recurringRulesLoading NOTIFY recurringRulesLoadingChanged)
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
    bool recordTransactionInProgress() const { return m_recordTransactionInProgress; }
    bool createRecurringRuleInProgress() const { return m_createRecurringRuleInProgress; }
    QVariantList recurringRules() const { return m_recurringRules; }
    bool recurringRulesLoading() const { return m_recurringRulesLoading; }
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
    Q_INVOKABLE void recordTransaction(const QString& accountId, const QString& date,
                                       qlonglong amount, const QString& name,
                                       const QString& description, int status);
    // Takes a params object rather than positional arguments: the call carries ten fields, and
    // named keys remove the transpose hazard that many positional arguments invite. The
    // required keys are enumerated (and enforced) in the implementation.
    Q_INVOKABLE void createRecurringRule(const QVariantMap& params);
    Q_INVOKABLE void listRecurringRules(const QString& accountId);
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
    void recordTransactionInProgressChanged();
    void transactionRecorded(QString id);
    void transactionRecordFailed(QString message);
    void createRecurringRuleInProgressChanged();
    void recurringRuleCreated(QString id);
    void recurringRuleCreateFailed(QString message);
    void recurringRulesChanged();
    void recurringRulesLoadingChanged();
    void timeSeriesLoadingChanged();
    void timeSeriesDataChanged();

private slots:
    void onPingFinished();
    void applyPingResult();
    void onListAccountsFinished();
    void onCreateAccountFinished();
    void onListTransactionsFinished();
    void onRecordTransactionFinished();
    void onCreateRecurringRuleFinished();
    void onListRecurringRulesFinished();
    void onTimeSeriesFinished();

private:
    // Starts a transactions fetch for m_requestedTransactionsAccountId (latching it as the
    // in-flight account). The re-entrancy guard in listTransactions and the reconciliation in
    // onListTransactionsFinished both funnel through here.
    void startTransactionsFetch();
    // Starts a recurring-rules fetch for m_requestedRecurringRulesAccountId. The re-entrancy
    // guard in listRecurringRules and the reconciliation in onListRecurringRulesFinished funnel
    // through here — the account-id-keyed twin of startTransactionsFetch.
    void startRecurringRulesFetch();

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
    // Single-in-flight reconciliation: the account the running fetch is for vs. the account
    // most recently requested. When they diverge (a rapid account switch arrived mid-fetch),
    // the stale result is discarded and the requested account re-fetched. Keyed on the
    // account id, not a bool like the accounts path, because the correct reconciled fetch
    // depends on which account is now selected.
    QString m_requestedTransactionsAccountId;
    QString m_inFlightTransactionsAccountId;
    QFutureWatcher<ListTransactionsResult> m_transactionsWatcher;
    bool m_recordTransactionInProgress = false;
    QFutureWatcher<RecordTransactionResult> m_recordTransactionWatcher;
    bool m_createRecurringRuleInProgress = false;
    QFutureWatcher<CreateRecurringRuleResult> m_createRecurringRuleWatcher;
    QVariantList m_recurringRules;
    bool m_recurringRulesLoading = false;
    // Account-id-keyed single-in-flight reconciliation, like the transactions path: the account
    // the running fetch is for vs. the account most recently requested.
    QString m_requestedRecurringRulesAccountId;
    QString m_inFlightRecurringRulesAccountId;
    QFutureWatcher<ListRecurringRulesResult> m_recurringRulesWatcher;
    bool m_timeSeriesLoading = false;
    QFutureWatcher<TimeSeriesResult> m_timeSeriesWatcher;
    TimeSeriesResult m_timeSeriesData;
};

#endif // FINCHCLIENT_H
