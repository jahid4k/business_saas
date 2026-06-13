'use client';

import {BuggyComponent, CounterBuggyComponent} from "@/app/page";

const TestPage = () => {
    return (
        <div>
            TEst page

            <BuggyComponent />

            <CounterBuggyComponent />
        </div>
    )
}

export default TestPage;