// agent-reconnect job has two purposes:
// 1. Reconnect machines that are disconnected.
//    It sometimes happens that machines get disconnected from the master. The main culprit for this
//    is Windows Update running on an agent machine. When this pipeline finds a disconnected
//    machine, it will attempt to reconnect it. It will also logs to cloud logging, in the
//    "jenkins-reconnect" log that this happened. You can then use that log to see when Jenkins
//    detected that a machine went offline.
//
// 2. Start new agents if there are less than the desired ones.
//    This pipeline sets a `LABEL_TARGETS` map that specify how many machines for each target should
//    be running. If there are less than that, this pipelines provisions as many machines needed for
//    that label to match configuration.
//    Note that an offline machine will count as an existing one.
//    It will also log into the "jenkins-reconnect" cloud log that this happened.
//    Note that this will not complain if there are *more* machines that the ones required. This
//    only enforces a minimum.

properties([
    parameters([
        string(name: "cloudProvider",
               description: "The cloud provider to use (most likely you want the default)",
               defaultValue: "gce-gcp"),
    ])
])

// This defines how many machines of each labels should be running.
def LABEL_TARGETS = [
    "experimental": 1,
    "presubmit": 2,
    "publish": 1,
    "cron": 1,
    "unit-runner": 3,
    "postsubmit": 1,
]

// https://wiki.jenkins.io/display/JENKINS/Monitor+and+Restart+Offline+Slaves
def IsAgentAccessible(computer) {
    return computer.environment?.get("PATH") != null
}

// Goes over all machines and checks if they are online. If offline, it will attempt to reconect.
// Returns how many offlines machines were detected (regardless if the reconnect succeeded or not).
def CheckComputers() {
    def numberNodes = 0
    def numberOfflineNodes = 0
    def offlineComputers = []
    for (agent in Hudson.getInstance().getNodes()) {
        def computer = agent.computer
        numberNodes++

        def ok = !computer.offline && IsAgentAccessible(computer)
        if (ok) {
            continue
        }

        numberOfflineNodes++
        def isOffline = agent.getComputer().isOffline()
        def tempOffline = agent.getComputer().isTemporarilyOffline()
        println("""
            Computer ${computer.name} OFFLINE!
            \tcomputer.isOffline: ${isOffline}
            \tcomputer.isTemporarilyOffline: ${tempOffline}
        """)

        //agent.getComputer().setTemporarilyOffline(true, null)
        //agent.getComputer().doDoDelete()
        // The boolean is whether a it should force reconnect and interrupt the
        // current task.
        println("Reconnecting...")
        agent.getComputer().connect(false)
        offlineComputers << computer.name
    }

    return offlineComputers
}

// Compares the current machines with the desired configuration (map of label -> number) and
// provisions more machines if needed.
def StartNewAgents(targets) {
    def provisions = []
    targets.each {
        def label = Jenkins.instance.getLabel(it.key)
        if (label == null) {
            error("Could not get label: ${it.key}")
        }
        // We need to do this annoying iteration because for some reason
        // getNodes returns a list that throws NullPointerException when
        // you access any method... like length :(
        int nodeCount = 0
        label.getNodes().each {
            nodeCount++
        }
        // Check if we need any more nodes.
        target = it.value
        if (nodeCount >= target) {
            println("Enough nodes for label ${it.key}")
            return
        }
        def gcp = Jenkins.instance.getCloud(params.cloudProvider)
        if (gcp == null) {
            error("Could not obtain cloud")
        }
        def configs = gcp.getInstanceConfigurations(label)
        if (configs == null) {
            error("Could not get worker configuration for given label: ${env.WORKER_LABEL}")
        }
        def config = configs[0]
        if (config == null) {
            error("Could not get worker configuration for given label: ${env.WORKER_LABEL}")
        }
        // Do the provision
        for (int i = nodeCount; i < target; i++) {
            println("Provisioning ${it.key}:${i}")
            provisions << "${it.key}:${i}"
            def instance = config.provision()
            if (instance == null) {
                error("Could not provision instance")
            }
            Jenkins.instance.addNode(instance)
        }
    }
    return provisions
}

pipeline {
    agent { node { label "master" }}
    stages {
        stage("Delete Offline Agents") {
            steps {
                script {
                    def offlineComputers = CheckComputers()
                    offlineComputers.each {
                        print("COMPUTER: $it")
                        sh """
                            gcloud logging write $LOG_NAME "Reconnected computer: $it"
                        """
                    }
                }
            }
        }
        stage("Start New Agents") {
            steps {
                script {
                    def provisions = StartNewAgents(LABEL_TARGETS)
                    provisions.each {
                        print("PROVISION: $it")
                        sh """
                            gcloud logging write $LOG_NAME "Provisioning: $it"
                        """
                    }
                }
            }
        }
    }
}
