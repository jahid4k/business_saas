'use client';

import {ReactNode} from "react";
import ErrorBoundary from "@/utils/ErrorBoundary";

type AppProps = {
    children?: ReactNode;
}

const App = (props: AppProps) => {
    const { children } = props;
    // const AppContextValue = {};

    return (
        <ErrorBoundary>
            {children}
        </ErrorBoundary>
    )
}

export default App;

// TODO: ErrorBoundary = DONE
// TODO: AppContext
// TODO: LocalizationProvider
// TODO: QueryClientProvider
// TODO: FuseSettingsProvider
// TODO: I18Provider
// TODO: RootThemeProvider
// TODO: MainThemeProvider
// TODO: NavbarContextProvider
// TODO: NavigationContextProvider
// TODO: FuseDialogContextProvider
// TODO: SnackbarProvider
// TODO: QuickPanelProvider