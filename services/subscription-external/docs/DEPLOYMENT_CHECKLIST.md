# 🚨 CRITICAL DEPLOYMENT CHECKLIST
## Enhanced Resubscription System - 25M+ Charging Failed Subscriptions

**⚠️ WARNING: This system will process 25+ million records. Follow this checklist EXACTLY to prevent system failure.**

---

## 📋 Pre-Deployment Checklist

### ✅ **Environment Preparation**
- [ ] **Staging Environment Ready**
  - [ ] Database accessible and healthy
  - [ ] Service deployment pipeline configured
  - [ ] Monitoring tools installed (Prometheus, Grafana)
  - [ ] Log aggregation configured
  - [ ] Backup systems verified

- [ ] **Database Backup Created**
  - [ ] Full database backup completed
  - [ ] Backup verified and accessible
  - [ ] Rollback procedures documented
  - [ ] Backup retention policy confirmed

- [ ] **Team Prepared**
  - [ ] Operations team trained on new system
  - [ ] Customer service team briefed
  - [ ] Emergency contacts documented
  - [ ] Rollback team assigned

---

## 🗄️ **Phase 1: Database Migration (CRITICAL)**

### **Step 1: Pre-Migration Validation**
```bash
# Run from services/subscription-external/scripts/
./validate_and_apply_migration.sh
```

**Expected Results:**
- [ ] Database connectivity verified
- [ ] Current schema analyzed
- [ ] Migration requirements identified
- [ ] Backup created successfully

### **Step 2: Migration Application**
```bash
# If migration is needed, run:
./validate_and_apply_migration.sh
```

**Validation Points:**
- [ ] All required tables created
- [ ] Indexes created successfully
- [ ] Functions and triggers installed
- [ ] Sample data test passed
- [ ] Rollback script generated

### **Step 3: Post-Migration Verification**
```sql
-- Verify required tables exist
SELECT table_name FROM information_schema.tables 
WHERE table_name IN ('resubscription_tracking', 'resubscription_checkpoints');

-- Verify required columns exist
SELECT column_name FROM information_schema.columns 
WHERE table_name = 'subscriptions' 
AND column_name IN ('charging_failure_count', 'last_charging_failure_at', 'resubscribe_status');

-- Verify indexes exist
SELECT indexname FROM pg_indexes 
WHERE tablename IN ('resubscription_tracking', 'resubscription_checkpoints');
```

---

## 🚀 **Phase 2: Service Deployment**

### **Step 1: Staging Deployment**
```bash
# Deploy enhanced services to staging
# Ensure all new endpoints are accessible
```

**Endpoints to Verify:**
- [ ] `/health` - Service health
- [ ] `/api/v1/subscription-external/resubscribe/enhanced` - Enhanced resubscribe
- [ ] `/api/v1/subscription-external/charging-failures` - Charging failures
- [ ] `/api/v1/subscription-external/batch/progress` - Batch progress
- [ ] `/api/v1/subscription-external/batch/stop` - Emergency stop

### **Step 2: Service Integration Test**
```bash
# Run comprehensive integration test
./test_service_integration.sh
```

**Test Results Required:**
- [ ] All basic endpoints respond correctly
- [ ] Enhanced endpoints accept requests
- [ ] Error handling works properly
- [ ] Performance meets requirements (< 1 second response time)
- [ ] Database integration verified

### **Step 3: Configuration Validation**
- [ ] Rate limiting configured correctly
- [ ] Circuit breaker thresholds set
- [ ] Checkpoint intervals configured
- [ ] Worker pool sizes defined
- [ ] Logging levels appropriate

---

## 🧪 **Phase 3: Pilot Testing (CRITICAL)**

### **Step 1: Small Scale Test (100 records)**
```bash
# Test with minimal data
curl -X POST "http://localhost:8083/api/v1/subscription-external/resubscribe/enhanced" \
  -H "Content-Type: application/json" \
  -d '{
    "telco": "TEST",
    "entry_channel": "USSD",
    "product_ids": ["1"],
    "batch_size": 100,
    "max_workers": 5,
    "use_charging_failures": false,
    "dry_run": true
  }'
```

**Validation Points:**
- [ ] Request accepted (HTTP 202)
- [ ] Job ID returned
- [ ] Processing started
- [ ] No database errors
- [ ] Logs generated correctly

### **Step 2: Medium Scale Test (1,000 records)**
```bash
# Increase to 1,000 records
curl -X POST "http://localhost:8083/api/v1/subscription-external/resubscribe/enhanced" \
  -H "Content-Type: application/json" \
  -d '{
    "telco": "TEST",
    "entry_channel": "USSD",
    "product_ids": ["1"],
    "batch_size": 1000,
    "max_workers": 10,
    "use_charging_failures": false,
    "dry_run": true
  }'
```

**Validation Points:**
- [ ] Processing rate > 50 records/second
- [ ] Memory usage stable
- [ ] Database connections within limits
- [ ] Checkpoints created correctly
- [ ] Progress tracking working

### **Step 3: Charging Failures Test (100 records)**
```bash
# Test with actual charging failures
curl -X POST "http://localhost:8083/api/v1/subscription-external/resubscribe/enhanced" \
  -H "Content-Type: application/json" \
  -d '{
    "telco": "TEST",
    "entry_channel": "USSD",
    "product_ids": ["1"],
    "batch_size": 100,
    "max_workers": 5,
    "use_charging_failures": true,
    "dry_run": true
  }'
```

**Validation Points:**
- [ ] Charging failures identified correctly
- [ ] Processing logic works
- [ ] Tracking records created
- [ ] No duplicate processing
- [ ] Error handling works

---

## 📊 **Phase 4: Monitoring Setup**

### **Step 1: Prometheus Metrics**
- [ ] Metrics endpoint accessible
- [ ] Key metrics being collected:
  - [ ] Processing rate (records/second)
  - [ ] Error rate
  - [ ] Active workers
  - [ ] Queue depth
  - [ ] Response times

### **Step 2: Grafana Dashboards**
- [ ] Real-time processing dashboard
- [ ] Error analysis dashboard
- [ ] Performance metrics dashboard
- [ ] Alerting rules configured

### **Step 3: Log Aggregation**
- [ ] Structured logging enabled
- [ ] Log levels appropriate
- [ ] Log rotation working
- [ ] Error logs searchable

---

## 🚨 **Phase 5: Emergency Procedures**

### **Step 1: Emergency Stop Test**
```bash
# Test emergency stop functionality
curl -X POST "http://localhost:8083/api/v1/subscription-external/batch/stop" \
  -H "Content-Type: application/json" \
  -d '{"batch_id": "test_batch"}'
```

**Validation Points:**
- [ ] Stop command accepted
- [ ] Processing halted within 30 seconds
- [ ] Workers stopped gracefully
- [ ] Checkpoint saved
- [ ] Status updated correctly

### **Step 2: Rollback Procedure Test**
```bash
# Test rollback script
psql -h localhost -U sm_admin -d subscription_manager \
  -f /tmp/migration_backups/rollback_*.sql
```

**Validation Points:**
- [ ] Rollback script executes without errors
- [ ] Database schema reverted
- [ ] Data integrity maintained
- [ ] Service continues to function

---

## 📈 **Phase 6: Production Rollout**

### **Step 1: Gradual Scale-Up**
1. **Week 1**: 10,000 records (0.04% of total)
2. **Week 2**: 100,000 records (0.4% of total)
3. **Week 3**: 1,000,000 records (4% of total)
4. **Week 4**: 5,000,000 records (20% of total)
5. **Week 5**: Remaining records

### **Step 2: Monitoring During Rollout**
- [ ] Real-time progress monitoring
- [ ] Error rate < 5%
- [ ] Processing rate > 100 records/second
- [ ] System resources within limits
- [ ] Customer complaints monitored

### **Step 3: Checkpoint Verification**
- [ ] Checkpoints created every 1,000 records
- [ ] Recovery from checkpoints tested
- [ ] Progress tracking accurate
- [ ] No data loss during failures

---

## ✅ **Success Criteria**

### **Technical Metrics**
- [ ] Processing rate: 100-200 records/second
- [ ] Error rate: < 5%
- [ ] System uptime: > 99.9%
- [ ] Response time: P95 < 2 seconds
- [ ] Database connections: < 80% of limit

### **Business Metrics**
- [ ] Revenue recovery: > 90% of failed subscriptions
- [ ] Customer retention: > 95%
- [ ] Processing completion: > 95% within timeline
- [ ] Zero data corruption incidents

---

## 🚨 **Critical Failure Points**

### **DO NOT PROCEED if:**
- [ ] Database migration fails
- [ ] Service integration tests fail
- [ ] Pilot test with 1,000 records fails
- [ ] Emergency stop doesn't work
- [ ] Monitoring is not functional
- [ ] Rollback procedures untested

### **IMMEDIATE STOP if:**
- [ ] Error rate exceeds 10%
- [ ] Processing rate drops below 50 records/second
- [ ] Database connections exceed 90%
- [ ] Memory usage exceeds 80%
- [ ] Customer complaints increase significantly

---

## 📞 **Emergency Contacts**

### **Technical Escalation**
1. **Primary**: [Operations Lead Name] - [Phone]
2. **Secondary**: [System Admin Name] - [Phone]
3. **Database**: [DBA Name] - [Phone]

### **Business Escalation**
1. **Product Manager**: [Name] - [Phone]
2. **Customer Service Lead**: [Name] - [Phone]
3. **Finance Team**: [Name] - [Phone]

---

## 📝 **Deployment Log**

| Phase | Status | Date | Notes |
|-------|--------|------|-------|
| Pre-Deployment | ⏳ Pending | | |
| Database Migration | ⏳ Pending | | |
| Service Deployment | ⏳ Pending | | |
| Pilot Testing | ⏳ Pending | | |
| Monitoring Setup | ⏳ Pending | | |
| Emergency Procedures | ⏳ Pending | | |
| Production Rollout | ⏳ Pending | | |

---

## 🎯 **Next Steps After Deployment**

1. **Immediate (Day 1)**
   - Monitor system health
   - Verify all endpoints working
   - Check monitoring dashboards

2. **Short Term (Week 1)**
   - Run pilot test with 1,000 records
   - Analyze performance metrics
   - Adjust configuration if needed

3. **Medium Term (Month 1)**
   - Begin gradual rollout
   - Monitor customer impact
   - Optimize performance

4. **Long Term (Month 2+)**
   - Complete 25M record processing
   - Analyze business impact
   - Plan future improvements

---

**⚠️ REMEMBER: This system will process 25+ million records. Safety and monitoring are paramount. When in doubt, STOP and investigate.** 